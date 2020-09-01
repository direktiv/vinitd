package main

import (
	"fmt"
	"os"

	"github.com/vorteil/vinitd/internal/vorteil"
	"golang.org/x/sys/unix"
)

var (
	vinitd *vorteil.Vinitd
)

type seqFn func() error

type seq struct {
	name string
	fn   seqFn
}

// Reboot and poweroff links
const (
	AppPoweroff = "/sbin/poweroff"
	AppReboot   = "/sbin/reboot"
)

func main() {

	// these are hard-coded in linux so we need to handle it
	if os.Args[0] == AppPoweroff || os.Args[0] == AppReboot {

		var file, err = os.OpenFile("/dev/vtty", os.O_RDWR, 0644)
		if err != nil {
			// what can we do....
			return
		}

		p, err := os.FindProcess(1)
		if err != nil {
			fmt.Fprintf(file, "can not find process 1: %s", err.Error())
			return
		}

		signal := unix.SIGINT
		if os.Args[0] == AppPoweroff {
			signal = unix.SIGPWR
		}

		err = p.Signal(signal)
		if err != nil {
			fmt.Fprintf(file, "can not send signal to process 1: %s", err.Error())
			return
		}

		return
	}

	vinitd = vorteil.New(vorteil.LogFnKernel)

	ss := []seq{
		{
			name: "pre-setup",
			fn:   vinitd.PreSetup,
		},
		{
			name: "setup",
			fn:   vinitd.Setup,
		},
		{
			name: "post-setup",
			fn:   vinitd.PostSetup,
		},
		{
			name: "launch",
			fn:   vinitd.Launch,
		},
	}

	for _, s := range ss {
		vorteil.LogFnKernel(vorteil.LOG_DEBUG, "starting seq %s", s.name)
		err := s.fn()
		if err != nil {
			vorteil.SystemPanic("can not run %s: %s", s.name, err.Error())
		}
	}

	select {}

}
