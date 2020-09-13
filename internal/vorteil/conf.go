package vorteil

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/vorteil/vorteil/pkg/vcfg"
	"github.com/vorteil/vorteil/pkg/vimg"
)

const (
	defaultTTY    = "/dev/vtty"
	defaultNrProc = 10000
	defaultCWD    = "/"

	getRequest      = "GET"
	azureWireServer = "168.63.129.16"          // NOSONAR known cloud
	metadataURL     = "http://169.254.169.254" // NOSONAR known cloud

	minFds = 1024
)

type cloudReq struct {
	server string

	interfaceURL  string
	customDataURL string
	hostnameURL   string

	header, query map[string]string
}

var (
	azureReq = cloudReq{
		server:        metadataURL,
		interfaceURL:  "%s/metadata/instance/network/interface/%d/ipv4/ipAddress/0/publicIpAddress",
		customDataURL: "%s/metadata/instance/compute/customData",
		hostnameURL:   "",
		header: map[string]string{
			"Metadata": "True",
			"Host":     "metadata.azure.internal",
		},
		query: map[string]string{
			"format":      "text",
			"api-version": "2019-02-01",
		},
	}

	gcpReq = cloudReq{
		server:        metadataURL,
		interfaceURL:  "%s/computeMetadata/v1/instance/network-interfaces/%d/access-configs/0/external-ip",
		customDataURL: "%s/computeMetadata/v1/instance/attributes/vorteil",
		hostnameURL:   "%s/computeMetadata/v1/instance/hostname",
		header: map[string]string{
			"Host":            "metadata.google.internal",
			"Metadata-Flavor": "Google",
		},
	}

	ec2Req = cloudReq{
		server:        metadataURL,
		interfaceURL:  "%s/latest/meta-data/public-ipv4",
		customDataURL: "%s/latest/user-data",
		hostnameURL:   "%s/latest/meta-data/public-hostname",
		header: map[string]string{
			"Host":     "metadata.ec2.internal",
			"Metadata": "true",
		},
	}
)

func bootDisk() (string, error) {

	b, err := ioutil.ReadFile(bootdev)
	if err != nil {
		return "", err
	}
	return string(b), nil

}

/* readVCFG reads the the configuration for the VM from disk into the
   configuration struct */
func (v *Vinitd) readVCFG(disk string) error {

	logDebug("reading vcfg from disk %s", disk)

	var (
		blc  vimg.BootloaderConfig
		vcfg vcfg.VCFG
	)

	f, err := os.Open(disk)
	if err != nil {
		return err
	}
	defer f.Close()

	off, err := f.Seek(vcfgOffset, io.SeekStart)
	if err != nil {
		return err
	}

	if off != vcfgOffset {
		return fmt.Errorf("can not read vcfg, wrong offset")
	}

	// var conf PersistedConf
	err = binary.Read(f, binary.LittleEndian, &blc)
	if err != nil {
		return err
	}

	logDebug("kernel args: %s", string(blc.LinuxArgs[:]))

	_, err = f.Seek((int64)(vcfgOffset+blc.ConfigOffset), io.SeekStart)
	if err != nil {
		return err
	}

	logDebug("config offset %d bytes", blc.ConfigOffset)

	vb := make([]byte, blc.ConfigLen)
	_, err = f.Read(vb)
	if err != nil {
		return err
	}

	err = json.Unmarshal(vb, &vcfg)
	if err != nil {
		return err
	}

	v.vcfg = vcfg

	// we need to set the user here
	v.user = vcfg.System.User

	return nil
}

func setupSharedMemory() error {

	var s string

	cmd, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return err
	}

	fmt.Sscanf(string(cmd), "shm=%s ", &s)

	if len(s) > 0 {

		err := mountFs("/dev/shm", "tmpfs", fmt.Sprintf("size=%s", s))
		if err != nil {
			return err
		}

	}

	return nil
}

func procsys(k string, val string) error {

	p := fmt.Sprintf("/proc/sys/%s", k)

	err := ioutil.WriteFile(p, []byte(val), 0644)
	if err != nil {
		return err
	}

	// double check if value has been accepted
	result, err := ioutil.ReadFile(p)
	if err != nil {
		return err
	}

	if val != strings.TrimSpace(string(result)) {
		return fmt.Errorf("values mismatch after set %s != %s", val, strings.TrimSpace(string(result)))
	}

	return nil

}

func rlimit(k int, v uint64) error {

	var rLimit syscall.Rlimit
	rLimit.Max = v
	rLimit.Cur = v

	return syscall.Setrlimit(k, &rLimit)

}

func uuidHasEc2() bool {

	uuid, err := ioutil.ReadFile("/sys/hypervisor/uuid")

	logDebug("uuid value: %s", strings.TrimSpace(string(uuid)))

	if err == nil && strings.HasPrefix(string(uuid), "ec2") {
		return true
	}

	return false
}

func (hv hv) cloudString() string {
	return cloudStrings[hv.cloud]
}

func (hv hv) hypervisorString() string {
	return hypervisorStrings[hv.hypervisor]
}

// azure needs this function to accept it as a running vm
func updateHealthAzure() {

	logDebug("update azure health")

	client := &http.Client{}
	req, _ := http.NewRequest(getRequest, fmt.Sprintf("http://%s/machine/", azureWireServer), nil)
	req.Header.Add("x-ms-agent-name", "WALinuxAgent")
	req.Header.Add("x-ms-version", "2012-11-30")

	q := req.URL.Query()
	q.Add("comp", "goalstate")
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		logError("error updating machine status: %s", err.Error())
		return
	}

	defer resp.Body.Close()
	r, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode == 200 && err == nil {

		var (
			cid string
			iid string
		)

		re := regexp.MustCompile(`<ContainerId>([^<]*)</ContainerId>`)
		result := re.FindAllStringSubmatch(string(r), -1)

		if result[0] == nil {
			logError("can not report VM as healthy, no container-id")
		}
		cid = result[0][1]

		re = regexp.MustCompile(`<InstanceId>([^<]*)</InstanceId>`)
		result = re.FindAllStringSubmatch(string(r), -1)
		if result[0] == nil {
			logError("can not report VM as healthy, no instance-id")
		}
		iid = result[0][1]

		xml := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"utf-8\"?><Health xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xmlns:xsd=\"http://www.w3.org/2001/XMLSchema\"><GoalStateIncarnation>1</GoalStateIncarnation><Container><ContainerId>%s</ContainerId><RoleInstanceList><Role><InstanceId>%s</InstanceId><Health><State>Ready</State></Health></Role></RoleInstanceList></Container></Health>", cid, iid)

		req, _ = http.NewRequest("POST", fmt.Sprintf("http://%s/machine/", azureWireServer), strings.NewReader(xml))

		req.Header.Set("Content-Type", "text/xml;charset=utf-8")
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(xml)))

		req.Header.Add("x-ms-agent-name", "WALinuxAgent")
		req.Header.Add("x-ms-version", "2012-11-30")

		q := req.URL.Query()
		q.Add("comp", "health")
		req.URL.RawQuery = q.Encode()

		resp, err := client.Do(req)
		if err != nil {
			logError("error updating machine status: %s", err.Error())
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			logError("can not report VM as healthy, final post failed")
		}

	}

}

func doMetadataRequest(url string, header, query map[string]string) (string, error) {
	var (
		err error
	)

	client := &http.Client{}
	req, err := http.NewRequest(getRequest, url, nil)

	if err != nil {
		return "", err
	}

	for k, v := range header {
		req.Header.Add(k, v)
	}

	if len(query) > 0 {
		q := req.URL.Query()
		for k, v := range query {
			q.Add(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := client.Do(req)
	if err != nil {
		logWarn("error requesting metadata %s: %s", url, err.Error())
		return "", err
	}

	defer resp.Body.Close()
	respByte, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logWarn("error reading metadata %s: %s", url, err.Error())
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("metadata not found")
	}

	return strings.TrimSpace(string(respByte)), nil
}

func probe(creq cloudReq, v *Vinitd) {

	for _, ifc := range v.ifcs {

		var url string
		if v.hypervisorInfo.cloud == cpEC2 {
			url = fmt.Sprintf(creq.interfaceURL, creq.server)
		} else {
			url = fmt.Sprintf(creq.interfaceURL, creq.server, ifc.idx)
		}

		logDebug("probe ip url %s", url)
		r, err := doMetadataRequest(url, creq.header, creq.query)
		if err != nil {
			logWarn("error requesting metadata: %s", err.Error())
			continue
		}
		logDebug("setting metadata %s to %s", fmt.Sprintf(envExtIP, ifc.idx), r)
		v.hypervisorInfo.envs[fmt.Sprintf(envExtIP, ifc.idx)] = r

		// at the moment ec2 is only one network card
		if v.hypervisorInfo.cloud == cpEC2 {
			break
		}
	}

	url := fmt.Sprintf(creq.customDataURL, creq.server)
	logDebug("probe custom url %s", url)

	userdata, err := doMetadataRequest(url, creq.header, creq.query)

	if err != nil {
		logDebug("error requesting metadata vorteil: %s", err.Error())
	} else {
		logDebug("setting metadata userdata to %s", userdata)
		v.hypervisorInfo.envs[envUserData] = userdata
	}

	// azure doesnt have this value
	if len(creq.hostnameURL) > 0 {
		url := fmt.Sprintf(creq.hostnameURL, creq.server)
		logDebug("probe hostname url %s", url)
		hn, err := doMetadataRequest(url, creq.header, creq.query)
		if err != nil {
			logDebug("error requesting metadata hostname: %s", err.Error())
		} else {
			logDebug("setting metadata ENV_EXT_HOSTNAME %s", hn)
			v.hypervisorInfo.envs[envExtHostname] = hn
		}

	}

}

func basicEnv(v *Vinitd) {

	// set basics
	v.hypervisorInfo.envs[envHypervisor] = v.hypervisorInfo.hypervisorString()
	v.hypervisorInfo.envs[envCloudProvider] = v.hypervisorInfo.cloudString()

	v.hypervisorInfo.envs[envEthCount] = fmt.Sprintf("%d", len(v.ifcs))
	v.hypervisorInfo.envs[envHostname] = v.hostname
	v.hypervisorInfo.envs[envExtHostname] = v.hostname
	v.hypervisorInfo.envs[envUserData] = ""

	for _, ifc := range v.ifcs {
		v.hypervisorInfo.envs[fmt.Sprintf(envIP, ifc.idx)] = ifc.addr.IP.String()

		// we set the env variables with internal so they are never empty
		v.hypervisorInfo.envs[fmt.Sprintf(envExtIP, ifc.idx)] = ifc.addr.IP.String()
	}

}

func fetchCloudMetadata(v *Vinitd) {

	basicEnv(v)

	logDebug("cloud values: %s %s", v.hypervisorInfo.hypervisorString(), v.hypervisorInfo.cloudString())

	if v.hypervisorInfo.cloud == cpAzure {
		updateHealthAzure()
		probe(azureReq, v)
	} else if v.hypervisorInfo.cloud == cpGCP {
		probe(gcpReq, v)
	} else if v.hypervisorInfo.cloud == cpEC2 {
		probe(ec2Req, v)
	}
}

func hypervisorGuess(v *Vinitd, bios string) (hypervisor, cloud) {

	logDebug("guessing hypervisor: %s", strings.TrimSpace(bios))

	if strings.HasPrefix(bios, "SeaBIOS") {
		return hvKVM, cpNone
	} else if strings.HasPrefix(bios, "innotek GmbH") {
		return hvVBox, cpNone
	} else if strings.HasPrefix(bios, "Phoenix Technologies LTD") {
		// start guestinfo vmtools
		startVMTools(len(v.ifcs), v.hostname)
		return hvVMWare, cpNone
	} else if strings.HasPrefix(bios, "Google") {
		return hvKVM, cpGCP
	} else if strings.HasPrefix(bios, "Amazon") {
		return hvKVM, cpEC2
	} else if strings.HasPrefix(bios, "Xen") {
		if uuidHasEc2() {
			return hvXen, cpEC2
		}
		return hvXen, cpNone
	} else if strings.HasPrefix(bios, "American Megatrends Inc.") {
		// the cloud value has been set by DHCP already, option 245
		cp := cpNone
		if v.hypervisorInfo.cloud == cpAzure {
			cp = cpAzure
		}
		return hvHyperV, cp
	}

	return hvUnknown, cpUnknown

}

func enableContainers() error {

	logDebug("mounting cgroups")

	// enable
	if syscall.Mount("cgroup", "/sys/fs/cgroup", "tmpfs", 0, "uid=0,gid=0,mode=0755") != nil {
		return fmt.Errorf("can not mount cgroup")
	}

	file, err := os.Open("/proc/cgroups")
	if err != nil {
		return err
	}
	defer file.Close()

	s := bufio.NewScanner(file)

	var name string
	var a, b, c int

	for s.Scan() {
		fmt.Sscanf(s.Text(), "%s\\t%d\\t%d\\t%d", &name, &a, &b, &c)
		if !strings.HasPrefix(name, "#") {
			os.MkdirAll(fmt.Sprintf("/sys/fs/cgroup/%s", name), 0755)
			err := syscall.Mount("cgroup", fmt.Sprintf("/sys/fs/cgroup/%s", name), "cgroup", 0, name)
			if err != nil {
				logError("can not mount cgroups: %s", err.Error())
			}
		}
	}

	if s.Err() != nil {
		return err
	}

	return nil
}

func systemConfig(sysctls map[string]string, hostname string, maxFds int) error {

	os.Remove("/etc/ld.so.preload")
	os.Remove("/etc/ld.so.cache")

	os.Chmod("/dev/sda", 0755)

	err := enableContainers()
	if err != nil {
		return err
	}
	// setting up shared memory if defined in kernel_args
	err = setupSharedMemory()
	if err != nil {
		return err
	}

	rlimit(unix.RLIMIT_NPROC, defaultNrProc)
	logDebug("setting max procs to %d", defaultNrProc)

	// make sure fds are at least 1024
	maxFds = max(minFds, maxFds)
	logDebug("setting max-fds to %d", maxFds)

	rlimit(unix.RLIMIT_NOFILE, uint64(maxFds*2))

	type sysVal struct {
		name  string
		value int
	}

	vals := []sysVal{
		{"fs/file-max", int(maxFds)},
		{"vm/max_map_count", 1048575},
		{"vm/swappiness", 0},
		{"kernel/randomize_va_space", 2},
		{"net/ipv4/tcp_no_metrics_save", 1},
		{"net/core/netdev_max_backlog", 5000},
		{"vm/dirty_background_ratio", 20},
		{"vm/dirty_ratio", 25},
		{"fs/protected_hardlinks", 1},
		{"fs/protected_symlinks", 1},
		{"fs/suid_dumpable", 1},
		{"kernel/kptr_restrict", 1},
		{"kernel/dmesg_restrict", 1},
		{"kernel/unprivileged_bpf_disabled", 1},
		{"net/ipv4/conf/all/bootp_relay", 0},

		{"net/ipv4/tcp_syncookies", 1},
		{"net/ipv4/tcp_syn_retries", 2},
		{"net/ipv4/tcp_synack_retries", 2},
		{"net/ipv4/tcp_max_syn_backlog", 4096},

		{"net/ipv4/ip_forward", 0},
		{"net/ipv4/conf/all/forwarding", 0},
		{"net/ipv4/conf/default/forwarding", 0},
		{"net/ipv6/conf/all/forwarding", 0},
		{"net/ipv6/conf/default/forwarding", 0},

		{"net/ipv4/conf/all/rp_filter", 1},
		{"net/ipv4/conf/default/rp_filter", 1},

		{"net/ipv4/conf/all/accept_redirects", 0},
		{"net/ipv4/conf/default/accept_redirects", 0},
		{"net/ipv4/conf/all/secure_redirects", 0},
		{"net/ipv4/conf/default/secure_redirects", 0},
		{"net/ipv6/conf/all/accept_redirects", 0},
		{"net/ipv6/conf/default/accept_redirects", 0},

		{"net/ipv4/conf/all/accept_source_route", 0},
		{"net/ipv4/conf/default/accept_source_route", 0},
		{"net/ipv6/conf/all/accept_source_route", 0},
		{"net/ipv6/conf/default/accept_source_route", 0},

		{"net/ipv4/conf/all/proxy_arp", 0},
		{"net/ipv4/conf/all/arp_ignore", 1},
		{"net/ipv4/conf/all/arp_announce", 2},

		{"net/ipv4/conf/default/log_martians", 0},
		{"net/ipv4/conf/all/log_martians", 0},

		{"net/ipv4/icmp_ignore_bogus_error_responses", 0},
		{"net/ipv4/icmp_echo_ignore_broadcasts", 1},
	}

	for _, s := range vals {
		err := procsys(s.name, fmt.Sprintf("%d", s.value))
		if err != nil {
			logError("can not set %s: %s", s.name, err.Error())
		}
	}

	kernelHostname := "kernel/hostname"
	err = procsys(kernelHostname, hostname)
	if err != nil {
		logError("can not set %s: %s", kernelHostname, err.Error())
	}

	for k, x := range sysctls {
		k = strings.Replace(k, ".", "/", -1)
		err := procsys(k, x)
		if err != nil {
			logError("can not set sysctl %s to %v", k, x)
		}
	}

	return nil

}
