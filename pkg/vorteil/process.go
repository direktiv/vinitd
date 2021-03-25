/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	ps "github.com/mitchellh/go-ps"
	"golang.org/x/sys/unix"
)

const (
	procCNMCASTListen = 1
	cnIDXProc         = 1
	cnValProc         = 1
	procEventExit     = 0x80000000
	procEventFork     = 0x00000001
	procEventExec     = 0x00000002

	busboxScript = "/vorteil/busybox"
)

var (
	procs    map[uint32]uint32
	internal map[uint32]string

	instantShutdown = false
	isFirecracker   = false
)

// ProcEventHeader ...
type ProcEventHeader struct {
	What        uint32
	CPU         uint32
	Timestamp   uint64
	ProcessPid  uint32
	ProcessTgid uint32
}

// CnMsg ...
type CnMsg struct {
	ID    CbID
	Seq   uint32
	Ack   uint32
	Len   uint16
	Flags uint16
}

// CbID ...
type CbID struct {
	Idx uint32
	Val uint32
}

func killAll() {

	pl, err := ps.Processes()
	if err != nil {
		logError("can not get processes: %s", err.Error())
		return
	}

	// iterate through all processes and send signals
	// most processes are ok with either SIGINT or SIGTERM
	for x := range pl {
		p := pl[x]

		// don't kill us (pid 1) and kthread (pid 2)
		if p.Pid() > 2 && p.PPid() > 2 {
			syscall.Kill(p.Pid(), syscall.SIGINT)
			syscall.Kill(p.Pid(), syscall.SIGTERM)
		}

	}

}

/* shutdown of system. timeout in milliseconds
basically just calling on of these :
LINUX_REBOOT_CMD_POWER_OFF       = 0x4321fedc
LINUX_REBOOT_CMD_RESTART         = 0x1234567 */
func shutdown(cmd int) {

	if initStatus == statusPoweroff {
		return
	}

	initStatus = statusPoweroff

	logAlways("shutting down applications")

	if !instantShutdown {
		sendTerminateSignals()
	}

	killAll()

	logAlways("shutting down system")

	// Fixed Timeout - Allows for shutdown logs to be printed
	if !isFirecracker {
		time.Sleep(250 * time.Millisecond)
	}

	ioutil.WriteFile("/proc/sysrq-trigger", []byte("s"), 0644)
	ioutil.WriteFile("/proc/sysrq-trigger", []byte("u"), 0644)

	// flush disk
	p, err := bootDisk()
	if err != nil {
		logError(fmt.Sprintf("could not get disk name: %s", err.Error()))
	} else {
		flushDisk(p)
	}

	// firecracker needs reboot for poweroff
	if isFirecracker {
		syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
	} else {
		syscall.Reboot(cmd)
	}

}

func sendTerminateSignals() {
	var wg sync.WaitGroup

	// loop through program's terminate signals
	for p, termSig := range terminateSignals {
		wg.Add(1)
		go func(np *program, sig syscall.Signal) {
			defer wg.Done()

			// if program is finished skip
			if np.cmd == nil || np.cmd.Process == nil || np.cmd.ProcessState.ExitCode() >= 0 {
				return
			}

			logAlways("program[%d] pid[%d] - sending signal '%s'", np.progIndex, np.cmd.Process.Pid, sig)

			// send terminate to program
			if err := np.cmd.Process.Signal(sig); err != nil {
				logError("could not send terminate signal program %v, error: %v", np.cmd.Process.Pid, err)
			}

			// Wait for process to be exit...
			select {
			case <-np.exitChannel:
			}
		}(p, termSig)
	}

	// Wait for all processes to be terminated, or continue after timeout
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		logAlways("applications terminated")
	case <-time.After(terminateWait):
		logWarn("could not terminate all applications before timeout")
	}
}

func listenToProcesses(progs []*program) {

	procs = make(map[uint32]uint32)
	internal = make(map[uint32]string)

	sock, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM, unix.NETLINK_CONNECTOR)

	if err != nil {
		logError("socket for process listening failed: %s", err.Error())
		return
	}

	addr := &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: cnIDXProc, Pid: uint32(os.Getpid())}
	err = unix.Bind(sock, addr)

	if err != nil {
		logError("bind for process listening failed: %s", err.Error())
		return
	}

	err = send(sock, procCNMCASTListen)
	if err != nil {
		logError("send for process listening failed: %s", err.Error())
		return
	}

	for {
		p := make([]byte, 1024)

		nlmessages, err := recv(p, sock)

		if err != nil {
			logDebug("error receiving netlink message: %s", err.Error())
			continue
		}

		for _, m := range nlmessages {
			parseNetlinkMessage(m, progs)
		}
	}
}

func handleExit(progs []*program) {

	// count is not correct. it is just a marker that we should keep running
	count := 0
	rpo, _ := ps.Processes()
	for _, p := range rpo {
		if p.Pid() > 2 && p.PPid() > 2 &&
			p.Executable() != "chronyd" &&
			p.Executable() != "fluent-bit" {
			count++
		}
	}

	// count the ones bootstrapping or not started
	for _, p := range progs {

		// has not been started or stil running
		if p.cmd == nil {
			count++
		} else if p.cmd != nil && p.cmd.ProcessState == nil && !p.reaper { // if this has been reaped
			count++
		} else if p.cmd != nil && p.cmd.ProcessState != nil && !p.cmd.ProcessState.Exited() {
			count++
		}

	}

	if count == 0 {
		if initStatus != statusPoweroff {
			logAlways("no programs still running")
			instantShutdown = true
		}
		shutdown(syscall.LINUX_REBOOT_CMD_POWER_OFF)
	}

}

func parseNetlinkMessage(m syscall.NetlinkMessage, progs []*program) {
	if m.Header.Type == unix.NLMSG_DONE {
		buf := bytes.NewBuffer(m.Data)
		msg := &CnMsg{}
		hdr := &ProcEventHeader{}
		binary.Read(buf, binary.LittleEndian, msg)
		binary.Read(buf, binary.LittleEndian, hdr)

		switch hdr.What {
		case procEventExit:
			{
				logDebug("remove application %d", hdr.ProcessPid)
				handleExit(progs)
			}
		}
	}
}

func send(sock int, msg uint32) error {
	cnMsg := CnMsg{
		Ack: 0,
		Seq: 1,
	}
	destAddr := &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: cnIDXProc, Pid: 0} // the kernel
	header := unix.NlMsghdr{
		Len:   unix.NLMSG_HDRLEN + uint32(binary.Size(cnMsg)+binary.Size(msg)),
		Type:  uint16(unix.NLMSG_DONE),
		Flags: 0,
		Seq:   1,
		Pid:   uint32(os.Getpid()),
	}
	cnMsg.ID = CbID{Idx: cnIDXProc, Val: cnValProc}
	cnMsg.Len = uint16(binary.Size(msg))

	buf := bytes.NewBuffer(make([]byte, 0, header.Len))
	binary.Write(buf, binary.LittleEndian, header)
	binary.Write(buf, binary.LittleEndian, cnMsg)
	binary.Write(buf, binary.LittleEndian, msg)

	return unix.Sendto(sock, buf.Bytes(), 0, destAddr)
}

func recv(p []byte, sock int) ([]syscall.NetlinkMessage, error) {
	nr, from, err := unix.Recvfrom(sock, p, 0)

	if sockaddrNl, ok := from.(*unix.SockaddrNetlink); !ok || sockaddrNl.Pid != 0 {
		return nil, fmt.Errorf("can not create netlink sockaddr")
	}

	if err != nil {
		return nil, err
	}

	if nr < unix.NLMSG_HDRLEN {
		return nil, fmt.Errorf("number of bytes too small, received %d bytes", nr)
	}

	nlmessages, err := syscall.ParseNetlinkMessage(p[:nr])

	if err != nil {
		return nil, err
	}

	return nlmessages, nil
}

func runBusyboxScript() error {

	dirs := []string{"/bin", "/usr/bin"}

	if _, err := os.Stat(busboxScript); err == nil {

		for _, d := range dirs {
			os.MkdirAll(d, 0755)
		}

		out, err := exec.Command(busboxScript, "--list").Output()
		if err != nil {
			return err
		}

		apps := strings.Split(string(out), "\n")
		for _, app := range apps {
			if app != "[" && app != "[[" && len(app) > 0 {
				for _, d := range dirs {

					a := filepath.Join(d, app)

					// if there is one already we don't do it
					if _, err := os.Stat(a); os.IsNotExist(err) {
						err = os.Symlink(busboxScript, a)
						if err != nil {
							return err
						}
					}

				}
			}
		}

	}

	return nil

}
