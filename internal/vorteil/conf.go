package vorteil

import (
	"bufio"
	"bytes"
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
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/vorteil/vorteil/pkg/vcfg"
	"github.com/vorteil/vorteil/pkg/vimg"
)

type pFieldType int

const (
	PCONF_NULL       pFieldType = iota
	PCONF_BINARY                = iota
	PCONF_VARC                  = iota
	PCONF_VAR                   = iota
	PCONF_ARGC                  = iota
	PCONF_ARG                   = iota
	PCONF_STDOUT                = iota
	PCONF_STDERR                = iota
	PCONF_BOOTSTRAPC            = iota
	PCONF_BOOTSTRAP             = iota
	PCONF_LOGFILEC              = iota
	PCONF_LOGFILE               = iota
	PCONF_CWD                   = iota
	PCONF_PRIVILEGE             = iota
	PCONF_STRACE                = iota
)

const (
	defaultTTY    = "/dev/vtty"
	defaultNrProc = 10000
	defaultCWD    = "/"

	getRequest      = "GET"
	azureWireServer = "168.63.129.16"          // NOSONAR known cloud
	metadataURL     = "http://169.254.169.254" // NOSONAR known cloud
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

type ProgramConfField struct {
	Len   uint16
	Ptype uint16
}

// func parseBootstrap(off int, length uint16, buf []byte) *bootstrapInstruction {
//
// 	return nil

// var b = &bootstrapInstruction{
// 	btype: bFieldType(binary.LittleEndian.Uint16(buf[off:])),
// }
//
// to := off + int(unsafe.Sizeof(length)) // it is uint16 further down after type
//
// switch b.btype {
// // case BOOTSTRAP_SLEEP:
// // 	{
// // 		b.time = binary.LittleEndian.Uint32(buf[to:])
// // 		break
// // 	}
// default:
// 	{
// 		s := to
// 		for {
// 			b.args = append(b.args, terminatedNullString(buf[to:]))
// 			to += len(terminatedNullString(buf[to:])) + 1
// 			if to-s >= int(length-6) { // 6 is offset type and len
// 				break
// 			}
// 		}
// 	}
// }
//
// return b

// }

// func fixDefaults(p *program) {
// 	// if p.stdout == "" {
// 	// 	p.stdout = defaultTTY
// 	// }
// 	//
// 	// if p.stderr == "" {
// 	// 	p.stderr = defaultTTY
// 	// }
//
// 	// Treat empty cwd as "/" because calculatePath Filepath.Join functions break when joining empty cwd with relative path
// 	// if p.cwd == "" {
// 	// 	p.cwd = defaultCWD
// 	// }
// }

func parseProgram(buf []byte) (int, *program, error) {

	var in *bytes.Reader
	var pf ProgramConfField
	var err error

	totalLen := binary.LittleEndian.Uint32(buf)

	if totalLen == 0 {
		return 0, nil, nil
	}

	off := 4 // offset for totalLen
	p := &program{
		status: STATUS_SETUP,
	}

	for {
		in = bytes.NewReader(buf[off:])
		err = binary.Read(in, binary.LittleEndian, &pf)
		if err != nil {
			return 0, nil, fmt.Errorf("can not read config")
		}
		bo := off + int(unsafe.Sizeof(pf))
		switch pf.Ptype {
		case PCONF_BINARY:
			{
				p.path = terminatedNullString(buf[bo:])
			}
		case PCONF_VARC:
			{
				// p.env.count = binary.LittleEndian.Uint16(buf[bo:])
			}
		case PCONF_VAR:
			{
				// p.env.values = append(p.env.values, terminatedNullString(buf[bo:]))
			}
		case PCONF_ARGC:
			{
				// p.args.count = binary.LittleEndian.Uint16(buf[bo:])
			}
		case PCONF_ARG:
			{
				// p.args.values = append(p.args.values, terminatedNullString(buf[bo:]))
			}
		case PCONF_STDOUT:
			{
				// p.stdout = terminatedNullString(buf[bo:])
			}
		case PCONF_STDERR:
			{
				// p.stderr = terminatedNullString(buf[bo:])
			}
		case PCONF_BOOTSTRAPC:
			{
				// p.bootstrapc = binary.LittleEndian.Uint16(buf[bo:])
			}
		case PCONF_BOOTSTRAP:
			{
				// p.bootstraps = append(p.bootstraps, parseBootstrap(bo, pf.Len, buf))
			}
		case PCONF_LOGFILEC:
			{
				// p.logs.count = binary.LittleEndian.Uint16(buf[bo:])
			}
		case PCONF_LOGFILE:
			{
				// p.logs.values = append(p.logs.values, terminatedNullString(buf[bo:]))
			}
		case PCONF_CWD:
			{
				// p.cwd = terminatedNullString(buf[bo:])
			}
		case PCONF_PRIVILEGE:
			{
				// p.privilege = buf[bo]
			}
		case PCONF_STRACE:
			{
				// p.strace = buf[bo]
				// // Change permissions for strace binary, so it can run as non-root
				// if p.strace == 0x1 && (os.Chmod("/vorteil/strace", 0755) != nil) {
				// 	return 0, nil, fmt.Errorf("can not change strace file permission")
				// }
			}
		default:
			return 0, nil, fmt.Errorf("unknown field in program config")
		}

		off += int(pf.Len)

		// -20 is for the MD5 digest and zeroes
		if off >= int(totalLen-20) {
			break
		}

	}

	// fixDefaults(p)

	return int(totalLen), p, nil
}

// func (v *Vinitd) loadLoggings(f *os.File) error {

// load app/system logging config
// region := v.vcfg.Disk.LoggingConfig
// regionLen := region.Sectors * sectorSize
//
// buf := make([]byte, regionLen)
// _, err := f.ReadAt(buf, int64(region.Lba*sectorSize))
// if err != nil {
// 	return err
// }
//
// off := 0
//
// for {
//
// 	l := &logEntry{
// 		logType: logType(buf[off]),
// 	}
//
// 	if l.logType == 0 {
// 		break
// 	}
//
// 	off++ // type
// 	off++ // size
//
// 	for {
// 		s := terminatedNullString(buf[off:])
// 		off++ // skip the null terminator
// 		if len(s) == 0 {
// 			break
// 		}
// 		l.logStrings = append(l.logStrings, s)
// 		off += len(s)
// 	}
//
// 	v.logEntries = append(v.logEntries, l)
//
// }

// 	return nil
//
// }

// func (v *Vinitd) loadConfigs(vcfg vcfg.VCFG) error {
func (v *Vinitd) loadConfigs() error {

	// region := v.vcfg.Disk.InitdConfig
	// regionLen := region.Sectors * sectorSize
	//
	// f, err := os.Open(disk)
	// if err != nil {
	// 	return err
	// }
	// defer f.Close()
	//
	// var buf = make([]byte, regionLen)
	// _, err = f.ReadAt(buf, int64(region.Lba*sectorSize))
	// if err != nil {
	// 	return err
	// }
	// offset := 0
	// for {
	// 	o, p, err := parseProgram(buf[offset:])
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if p == nil {
	// 		break
	// 	}
	//
	// 	p.vinitd = v
	// 	v.programs = append(v.programs, p)
	//
	// 	if offset >= int(regionLen) || o == 0 {
	// 		break
	// 	}
	// 	offset += o
	// }
	//
	// // load go config
	// region = v.vcfg.Disk.GoConfig
	// regionLen = region.Sectors * sectorSize
	//
	// buf = make([]byte, regionLen)
	// _, err = f.ReadAt(buf, int64(region.Lba*sectorSize))
	// if err != nil {
	// 	return err
	// }
	//
	// jsonStr := terminatedNullString(buf)
	//
	// gocfg := new(GoCfg)
	// err = json.Unmarshal([]byte(jsonStr), gocfg)
	// if err != nil {
	// 	logError("could not parse goconfg settings")
	// } else {
	// 	v.sysctls = gocfg.Sysctl
	// 	v.user = gocfg.User
	// }
	//
	// return v.loadLoggings(f)

	return nil

}

func bootDisk() (string, error) {
	f, err := os.Open(bootdev)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 128)
	_, err = f.Read(buf)
	if err != nil {
		return "", err
	}

	// we need to read from the disk into the struct
	return terminatedNullString(buf), nil
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
		if v.hypervisorInfo.cloud == CP_EC2 {
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
		logDebug("setting metadata %s to %s", fmt.Sprintf(ENV_EXT_IP, ifc.idx), r)
		v.hypervisorInfo.envs[fmt.Sprintf(ENV_EXT_IP, ifc.idx)] = r

		// at the moment ec2 is only one network card
		if v.hypervisorInfo.cloud == CP_EC2 {
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
		v.hypervisorInfo.envs[ENV_USERDATA] = userdata
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
			v.hypervisorInfo.envs[ENV_EXT_HOSTNAME] = hn
		}

	}

}

func basicEnv(v *Vinitd) {

	// set basics
	v.hypervisorInfo.envs[ENV_HYPERVISOR] = v.hypervisorInfo.hypervisorString()
	v.hypervisorInfo.envs[ENV_CLOUD_PROVIDER] = v.hypervisorInfo.cloudString()

	v.hypervisorInfo.envs[ENV_ETH_COUNT] = fmt.Sprintf("%d", len(v.ifcs))
	v.hypervisorInfo.envs[ENV_HOSTNAME] = v.hostname
	v.hypervisorInfo.envs[ENV_EXT_HOSTNAME] = v.hostname
	v.hypervisorInfo.envs[ENV_USERDATA] = ""

	for _, ifc := range v.ifcs {
		v.hypervisorInfo.envs[fmt.Sprintf(ENV_IP, ifc.idx)] = ifc.addr.IP.String()

		// we set the env variables with internal so they are never empty
		v.hypervisorInfo.envs[fmt.Sprintf(ENV_EXT_IP, ifc.idx)] = ifc.addr.IP.String()
	}

}

func fetchCloudMetadata(v *Vinitd) {

	basicEnv(v)

	logDebug("cloud values: %s %s", v.hypervisorInfo.hypervisorString(), v.hypervisorInfo.cloudString())

	if v.hypervisorInfo.cloud == CP_AZURE {
		updateHealthAzure()
		probe(azureReq, v)
	} else if v.hypervisorInfo.cloud == CP_GCP {
		probe(gcpReq, v)
	} else if v.hypervisorInfo.cloud == CP_EC2 {
		probe(ec2Req, v)
	}
}

func hypervisorGuess(v *Vinitd, bios string) (hypervisor, cloud) {

	logDebug("guessing hypervisor: %s", strings.TrimSpace(bios))

	if strings.HasPrefix(bios, "SeaBIOS") {
		return HV_KVM, CP_NONE
	} else if strings.HasPrefix(bios, "innotek GmbH") {
		return HV_VIRTUALBOX, CP_NONE
	} else if strings.HasPrefix(bios, "Phoenix Technologies LTD") {
		// start guestinfo vmtools
		startVMTools(len(v.ifcs), v.hostname)
		return HV_VMWARE, CP_NONE
	} else if strings.HasPrefix(bios, "Google") {
		return HV_KVM, CP_GCP
	} else if strings.HasPrefix(bios, "Amazon") {
		return HV_KVM, CP_EC2
	} else if strings.HasPrefix(bios, "Xen") {
		if uuidHasEc2() {
			return HV_XEN, CP_EC2
		}
		return HV_XEN, CP_NONE
	} else if strings.HasPrefix(bios, "American Megatrends Inc.") {
		// the cloud value has been set by DHCP already, option 245
		cp := CP_NONE
		if v.hypervisorInfo.cloud == CP_AZURE {
			cp = CP_AZURE
		}
		return HV_HYPERV, cp
	}

	return HV_UNKNOWN, CP_UNKNOWN

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
