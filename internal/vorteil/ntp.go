package vorteil

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// NTP vars
var (
	chronydCfgData = `
driftfile /etc/chrony.drift
makestep 1.0 3
rtcsync`
	chronydCfgPath = "/etc/chrony.conf"
)

func setupChronyD(vNtpList [5]NTPSrv) error {
	ntpSrvList := make([]string, 0)
	for _, ntp := range vNtpList {
		if ntpString := terminatedNullString(ntp[:]); ntpString != "" {
			ntpSrvList = append(ntpSrvList, ntpString)
		}
	}

	// If server was found start chronyd
	if len(ntpSrvList) != 0 {
		logAlways("ntp servers\t: %s", strings.Join(ntpSrvList, ", "))
		if _, err := os.Stat(filepath.Dir(chronydCfgPath)); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(chronydCfgPath), 0755); err != nil {
				return fmt.Errorf("chronyd\t: could not create dir, %v", err)
			}
		}

		// Prepend servers to config data
		for _, ntpServer := range ntpSrvList {
			chronydCfgData = fmt.Sprintf("server %s iburst\n%s", ntpServer, chronydCfgData)
		}

		// Write config data
		if err := ioutil.WriteFile(chronydCfgPath, []byte(chronydCfgData), 0644); err != nil {
			return fmt.Errorf("chronyd\t: could not write config file, %v", err)
		}

		// Start ChronyD
		chronydCMD := exec.Command("/vorteil/chronyd") // set args to []string{"-l", "/etc/chrony.log"}... to save logs
		chronydCMD.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{Uid: uint32(rootID), Gid: uint32(rootID)},
		}

		err := chronydCMD.Start()
		if err != nil {
			return fmt.Errorf("chronyd\t: could not execute chronyd, %v", err)
		}
	}

	return nil
}
