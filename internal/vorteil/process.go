package vorteil

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	ps "github.com/mitchellh/go-ps"
	"golang.org/x/sys/unix"
)

const (
	PROC_CN_MCAST_LISTEN = 1
	CN_IDX_PROC          = 1
	CN_VAL_PROC          = 1
	PROC_EVENT_EXIT      = 0x80000000
	PROC_EVENT_FORK      = 0x00000001
	PROC_EVENT_EXEC      = 0x00000002

	busboxScript = "/vorteil/busybox-install.sh"
)

var (
	procs    map[uint32]uint32
	internal map[uint32]string
)

type ProcEventHeader struct {
	What        uint32
	CPU         uint32
	Timestamp   uint64
	ProcessPid  uint32
	ProcessTgid uint32
}

type CnMsg struct {
	ID    CbID
	Seq   uint32
	Ack   uint32
	Len   uint16
	Flags uint16
}

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
func shutdown(cmd, timeout int) {

	if initStatus == STATUS_POWEROFF {
		return
	}

	initStatus = STATUS_POWEROFF

	logAlways("shutting down applications")

	killAll()

	time.Sleep(time.Duration(timeout) * time.Millisecond)

	for i := 3; i > 0; i-- {
		logAlways(fmt.Sprintf("shutting down in %d...", i))
		time.Sleep(1 * time.Second)
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

	syscall.Reboot(cmd)
}

func listenToProcesses(progs []*program) {

	procs = make(map[uint32]uint32)
	internal = make(map[uint32]string)

	sock, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM, unix.NETLINK_CONNECTOR)

	if err != nil {
		logError("socket for process listening failed: %s", err.Error())
		return
	}

	addr := &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: CN_IDX_PROC, Pid: uint32(os.Getpid())}
	err = unix.Bind(sock, addr)

	if err != nil {
		logError("bind for process listening failed: %s", err.Error())
		return
	}

	err = send(sock, PROC_CN_MCAST_LISTEN)
	if err != nil {
		logError("send for process listening failed: %s", err.Error())
		return
	}

	for {
		p := make([]byte, 1024)

		nlmessages, err := recv(p, sock)

		if err != nil {
			logWarn("error receiving netlink message: %s", err.Error())
			continue
		}

		for _, m := range nlmessages {
			parseNetlinkMessage(m, progs)
		}
	}
}

func handleExit(hdr *ProcEventHeader, progs []*program) {
	if hdr.ProcessTgid == hdr.ProcessPid {

		// check if internal process
		if len(internal[hdr.ProcessTgid]) > 0 {
			delete(internal, hdr.ProcessTgid)
			return
		}

		delete(procs, hdr.ProcessTgid)
		if len(procs) == 0 {

			// check if all apps have started. they might be in bootstrap
			for _, p := range progs {
				if p.status == STATUS_SETUP {
					return
				}
			}

			logAlways("no programs still running")
			shutdown(syscall.LINUX_REBOOT_CMD_POWER_OFF, 0)
		}
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
		case PROC_EVENT_FORK:
		case PROC_EVENT_EXEC:
			{
				st, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", hdr.ProcessTgid))
				if err != nil {
					// app probably already finished
					return
				}

				if !strings.HasPrefix(st, "/vorteil/") || st == "/vorteil/busybox" {
					procs[hdr.ProcessTgid] = hdr.ProcessTgid
				} else {
					internal[hdr.ProcessTgid] = st
				}

				break
			}
		case PROC_EVENT_EXIT:
			{
				handleExit(hdr, progs)
			}
		}
	}
}

func send(sock int, msg uint32) error {
	cnMsg := CnMsg{
		Ack: 0,
		Seq: 1,
	}
	destAddr := &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: CN_IDX_PROC, Pid: 0} // the kernel
	header := unix.NlMsghdr{
		Len:   unix.NLMSG_HDRLEN + uint32(binary.Size(cnMsg)+binary.Size(msg)),
		Type:  uint16(unix.NLMSG_DONE),
		Flags: 0,
		Seq:   1,
		Pid:   uint32(os.Getpid()),
	}
	cnMsg.ID = CbID{Idx: CN_IDX_PROC, Val: CN_VAL_PROC}
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

	if _, err := os.Stat(busboxScript); err == nil {

		cmd := exec.Command(busboxScript)

		err = cmd.Start()
		if err != nil {
			return err
		}

		cmd.Wait()

	}

	return nil

}
