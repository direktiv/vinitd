package vorteil

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	sectorSize = 512
	bootdev    = "/proc/bootdev"

	// vcfg is on the disk 34 blocks in
	vcfgOffset = sectorSize * 34
	vcfgSize   = 0x4000

	forcedPoweroffTimeout = 3000
)

// New returns a new vinitd object
func New(logging logFn) *Vinitd {

	rand.Seed(time.Now().UnixNano())

	v := &Vinitd{
		ifcs: make(map[string]*ifc),
	}

	vlog = logging

	// hypervisor and vorteil special envse.g. IP_0, EXT_HOSTNAME
	v.hypervisorInfo.envs = make(map[string]string)

	return v

}

func setupMountOptions(diskname string) error {

	var (
		dev, path, fstype, opts string
		a, b                    int
	)

	// it is always the second partition
	part := fmt.Sprintf("%s2", diskname)

	file, err := os.Open("/proc/mounts")
	if err != nil {
		return err
	}
	defer file.Close()

	s := bufio.NewScanner(file)

	for s.Scan() {
		fmt.Sscanf(s.Text(), "%s %s %s %s %d %d", &dev, &path, &fstype, &opts, &a, &b)
		if path == "/" {
			logDebug("config %s filesystem on %s, %s", fstype, part, dev)
			switch fstype {
			case "ext2":
				{
					opts = ""
				}
			case "ext4":
				{
					opts = "nodiscard,commit=30,inode_readahead_blks=64"
				}
			case "xfs":
				{
					opts = "nodiscard,attr2,inode64,noquota"
				}
			default:
				{
					return fmt.Errorf("unknown filesystem format: %s", fstype)
				}
			}

			// MS_LAZYTIME 1 << 25
			logDebug("using fs opts %s", opts)
			return syscall.Mount(part, "/", fstype, syscall.MS_REMOUNT|syscall.MS_NOATIME|(1<<25), opts)

		}
	}

	if s.Err() != nil {
		return fmt.Errorf("could not detect filesystem type: %s", err.Error())
	}

	return fmt.Errorf("can not find root filesystem")
}

func (v *Vinitd) PreSetup() error {

	setupVtty(0)

	err := setupBasicDirectories()
	if err != nil {
		logError("error prep directories: %s", err.Error())
		return err
	}

	err = growDisks()
	if err != nil {
		return err
	}
	logDebug("pre-setup finished successfully")

	// fetch bootdisk from /proc/bootdev
	// the kernel has written the boot device into /dev/bootdevice
	// easier to figure out where to read from
	v.diskname, err = bootDisk()
	if err != nil {
		return err
	}

	// on error we can proceed here
	// has performance impact but can still run
	err = setupMountOptions(v.diskname)
	if err != nil {
		logError("can not setup mount options: %s", err.Error())
	}

	return nil

}

func (v *Vinitd) Setup() error {

	err := v.readVCFG(v.diskname)
	if err != nil {
		logWarn("error loading vcfg: %s", err.Error())
		return err
	}

	// update tty to settings in vcfg
	setupVtty(v.vcfg.System.StdoutMode)

	go waitForSignal()

	go changeDiskScheduler(v.diskname)

	// power functions
	go listenToPowerEvent()
	go prepSbinPower()

	syscall.Reboot(syscall.LINUX_REBOOT_CMD_CAD_OFF)
	printVersion()

	// generate hostname before running setup steps in parallel
	hn, err := setHostname(v.vcfg.SaltedHostname())
	if err != nil {
		logWarn("could not set hostname: %s", err.Error())
	} else {
		v.hostname = hn
	}
	logDebug("set hostname to %s", hn)

	errors := make(chan error)
	wgDone := make(chan bool)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		err = v.NetworkSetup()
		if err != nil {
			logError("error setting up network: %s", err.Error())
			errors <- err
		}
		wg.Done()
	}()

	go func() {
		err = systemConfig(v.vcfg.Sysctl, v.hostname, int(v.vcfg.System.MaxFDs))
		if err != nil {
			logError("can not setup shared memory: %s", err.Error())
		}
		wg.Done()
	}()

	go func() {
		err = etcGenerateFiles("/etc", v.hostname, v.user)
		if err != nil {
			logError("error creating etc files: %s", err.Error())
			errors <- err
		}
		wg.Done()
	}()

	logDebug("system setup waiting to finish")

	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		break
	case err := <-errors:
		close(errors)
		SystemPanic("sytem setup failed: %s", err.Error())
	}

	for _, p := range v.vcfg.Programs {
		v.prepProgram(p)
	}

	logDebug("system setup successful")

	return nil
}

func (v *Vinitd) PostSetup() error {

	// start a DNS on 127.0.0.1
	err := v.startDNS(defaultDNSAddr)

	// we might be able to run
	if err != nil {
		logWarn("can not start local DNS server")
	}

	errors := make(chan error)
	wgDone := make(chan bool)
	var wg sync.WaitGroup

	wg.Add(5)

	go func() {
		setupNFS(v.vcfg.NFS)
		wg.Done()
	}()

	go func() {
		if len(v.vcfg.Logging) > 0 {
			v.startLogging()
		}
		wg.Done()
	}()

	// get cloud information
	go func() {

		bios, err := ioutil.ReadFile("/sys/devices/virtual/dmi/id/bios_vendor")
		if err != nil {
			logWarn("can not read bios vendor")
			v.hypervisorInfo.hypervisor, v.hypervisorInfo.cloud = HV_UNKNOWN, CP_UNKNOWN
			wg.Done()
			return
		}

		v.hypervisorInfo.hypervisor, v.hypervisorInfo.cloud = hypervisorGuess(v, string(bios))
		fetchCloudMetadata(v)
		wg.Done()

	}()

	// prepare shell if --shell is provided
	go func() {
		err := runBusyboxScript()
		if err != nil {
			errors <- err
		}
		wg.Done()
	}()

	// Setup ChronyD NTP Server
	go func() {
		if err := setupChronyD(v.vcfg.System.NTP); err != nil {
			errors <- err
		}
		wg.Done()
	}()

	// wait on all tasks
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		break
	case err := <-errors:
		close(errors)
		SystemPanic("sytem post-setup failed: %s", err.Error())
	}

	logDebug("post setup finished successfully")
	initStatus = STATUS_RUN

	return nil
}

func waitForSignal() {

	killSignal := make(chan os.Signal, 1)
	signal.Notify(killSignal, syscall.SIGINT, syscall.SIGPWR)

	sig := <-killSignal

	logAlways("got signal %d", sig)
	if sig == syscall.SIGPWR {
		shutdown(syscall.LINUX_REBOOT_CMD_POWER_OFF, 0)
	} else {
		shutdown(syscall.LINUX_REBOOT_CMD_RESTART, 0)
	}

}
