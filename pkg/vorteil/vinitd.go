/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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

	fluxDir   = "/flux-data"
	fluxError = "error.out"
	fluxLog   = "log.out"
)

type networkSetting struct {
	IP      string `json:"ip"`
	Mask    string `json:"mask"`
	Gateway string `json:"gw"`
}

// flux disks
var uuidPlain = [16]byte{0x7d, 0x44, 0x48, 0x40, 0x9d, 0xc0, 0x11, 0xd1, 0xb2, 0x45, 0x5f, 0xfd, 0xce, 0x74, 0xfa, 0xd3}

// New returns a new vinitd object
func New() *Vinitd {

	rand.Seed(time.Now().UnixNano())

	v := &Vinitd{
		ifcs: make(map[string]*ifc),
	}

	// hypervisor and vorteil special envse.g. IP_0, EXT_HOSTNAME
	v.hypervisorInfo.envs = make(map[string]string)

	return v

}

func hasCmdLineString(ss string) bool {

	cmd, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return false
	}
	logDebug("cmdline: %s", string(cmd))
	logDebug("check cmdline for string: %v", ss)

	// check if it is set to ro
	for _, o := range strings.Split(string(cmd), " ") {
		if o == ss {
			logDebug("found string in cmdline %s", ss)
			return true
		}
	}

	return false
}

func setupMountOptions(diskname string, readOnly bool) error {

	var (
		dev, path, fstype, opts string
		a, b                    int
		err                     error
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

		logDebug("mount options %s", opts)

		// MS_LAZYTIME 1 << 25
		flags := syscall.MS_REMOUNT | syscall.MS_NOATIME | (1 << 25)

		if readOnly {
			flags |= syscall.MS_RDONLY
		}

		if path == "/" {
			logDebug("config %s filesystem on %s, %s", fstype, part, dev)
			switch fstype {
			case "ext2":
				{
					opts = "barrier=0"
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

			logDebug("using fs opts %s, flags %x", opts, flags)
			return syscall.Mount(part, "/", fstype, uintptr(flags), opts)

		}
	}

	err = fmt.Errorf("can not find root filesystem")

	if s.Err() != nil {
		err = fmt.Errorf("could not detect filesystem type: %s", err.Error())
	}
	return err

}

// PreSetup prepares directories, grows disk to max size and remounts disk
// with new args
func (v *Vinitd) PreSetup() error {

	v.setupVtty(0)

	err := setupBasicDirectories("/")
	if err != nil {
		logError("error prep directories: %s", err.Error())
		return err
	}

	go v.checkLogs()

	// fetch bootdisk from /proc/bootdev
	// the kernel has written the boot device into /dev/bootdevice
	// easier to figure out where to read from
	v.diskname, err = bootDisk()
	if err != nil {
		return err
	}

	err = v.readVCFG(v.diskname)
	if err != nil {
		logWarn("error loading vcfg: %s", err.Error())
		return err
	}

	v.readOnly = hasCmdLineString("ro")

	// on error we can proceed here
	// has performance impact but can still run
	err = setupMountOptions(v.diskname, v.readOnly)
	if err != nil {
		logError("can not setup mount options: %s", err.Error())
	}

	// mount /tmp as memory fs if read-only
	if hasCmdLineString("fluxsystem") {

		flags := syscall.MS_NOATIME | syscall.MS_SILENT

		// check if there is a second disk
		files, err := ioutil.ReadDir("/sys/block")
		if err == nil {

			for _, f := range files {

				// without loop and boot
				if !strings.HasPrefix(f.Name(), "loop") &&
					f.Name() != filepath.Base(v.diskname) {

					logDebug("checking disk for flux: %v", f.Name())

					// we are assuming gpt here
					d, err := os.Open(filepath.Join("/dev", f.Name()))
					if err != nil {
						continue
					}

					// first disk uid offset in lba 1
					var puid [16]byte
					_, err = d.ReadAt(puid[:], 512+56)

					if err != nil {
						continue
					}

					if bytes.Compare(uuidPlain[:], puid[:]) == 0 {

						disk := fmt.Sprintf("/dev/%s1", f.Name())
						logDebug("mounting %s to %s", disk, fluxDir)
						err = syscall.Mount(disk, fluxDir, "ext2", uintptr(flags), "barrier=0")
						if err != nil {
							SystemPanic("can not mount flux data disk: %v", err)
						}

						// change network setting
						nw, err := ioutil.ReadFile(filepath.Join(fluxDir, "network.in"))
						if err != nil {
							SystemPanic("can not read flux network settings: %v", err)
						}

						var nws networkSetting
						err = json.Unmarshal(nw, &nws)
						if err != nil {
							SystemPanic("can not read flux network settings json: %v", err)
						}

						cl := func(name string) {
							_, err = os.OpenFile(filepath.Join(fluxDir, name), os.O_CREATE|os.O_RDWR, 0755)
							if err != nil {
								SystemPanic("can not create error.log: %v", err)
							}
						}

						cl(fluxError)
						cl(fluxError)

						v.vcfg.Networks[0].IP = nws.IP
						v.vcfg.Networks[0].Gateway = nws.Gateway
						v.vcfg.Networks[0].Mask = nws.Mask

						instantShutdown = true

					}
				}
			}

		}
	}

	err = growDisks()
	if err != nil {
		return err
	}
	logDebug("pre-setup finished successfully")

	terminateWait = time.Duration(v.vcfg.System.TerminateWait) * time.Millisecond

	// Backwards Compatability
	//	- Should never happen unless old vorteil binary was used with new kernel
	if terminateWait == 0 {
		logWarn("system.terminate-wait is empty. This suggests vorteil binary is out of date")
		terminateWait = 3000 * time.Millisecond
	}

	return nil

}

// Setup is the main routine during preparing a vorteil machine.
// It prepares stdout, poweroff events, network and basic system configuration
func (v *Vinitd) Setup() error {

	// update tty to settings in vcfg
	logDebug("output mode: %v", v.vcfg.System.StdoutMode)
	v.setupVtty(v.vcfg.System.StdoutMode)

	go waitForSignal()

	go changeDiskScheduler(v.diskname)

	// power functions
	go listenToPowerEvent()

	if !v.readOnly {
		go prepSbinPower()
	}

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
		err = v.networkSetup()
		if err != nil {
			logError("error setting up network: %s", err.Error())
			errors <- err
		}
		wg.Done()
	}()

	go func() {
		err = systemConfig(v.vcfg.Sysctl, v.hostname, int(v.vcfg.System.MaxFDs))
		if err != nil {
			logError("can not setup basic config: %s", err.Error())
		}
		wg.Done()
	}()

	go func() {
		if !v.readOnly {
			err = etcGenerateFiles(v.hostname, v.user)
			if err != nil {
				logError("error creating etc files: %s", err.Error())
				errors <- err
			}
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
		SystemPanic("system setup failed: %s", err.Error())
	}

	for i, p := range v.vcfg.Programs {
		v.prepProgram(p, i)
	}

	logDebug("system setup successful")

	return nil
}

// PostSetup finishes tasks which need network access which is DNS, NFS and NTP
func (v *Vinitd) PostSetup() error {

	// start a DNS on 127.0.0.1
	basicEnv(v)
	err := v.startDNS(defaultDNSAddr, true)

	// we might be able to run
	if err != nil {
		logWarn("can not start local DNS server")
	}

	errors := make(chan error)
	wgDone := make(chan bool)
	var wg sync.WaitGroup

	wg.Add(5)
	cread := make(chan bool)

	go func() {
		setupNFS(v.vcfg.NFS)
		wg.Done()
	}()

	go func() {
		if len(v.vcfg.Logging) > 0 && !v.readOnly {
			// waiting for the cloud metadata
			<-cread
			v.startLogging()
		} else if len(v.vcfg.Logging) > 0 {
			logWarn("filesystem read-only, can not start logging")
		}
		wg.Done()
	}()

	// get cloud information
	go func() {

		defer func() {
			wg.Done()
			cread <- false
		}()

		bios, err := ioutil.ReadFile("/sys/devices/virtual/dmi/id/bios_vendor")
		if err != nil {
			logWarn("can not read bios vendor")
			v.hypervisorInfo.hypervisor, v.hypervisorInfo.cloud = hvUnknown, cpUnknown
			basicEnv(v)
			return
		}

		v.hypervisorInfo.hypervisor, v.hypervisorInfo.cloud = hypervisorGuess(v, string(bios))
		fetchCloudMetadata(v)

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
		SystemPanic("system post-setup failed: %s", err.Error())
	}

	logDebug("post setup finished successfully")
	initStatus = statusRun

	return nil
}

func waitForSignal() {

	killSignal := make(chan os.Signal, 1)
	signal.Notify(killSignal, syscall.SIGINT, syscall.SIGPWR)

	sig := <-killSignal

	logDebug("got signal %d", sig)
	if sig == syscall.SIGPWR {
		shutdown(syscall.LINUX_REBOOT_CMD_POWER_OFF)
	} else {
		shutdown(syscall.LINUX_REBOOT_CMD_RESTART)
	}

}
