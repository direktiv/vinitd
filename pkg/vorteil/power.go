/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func listenPowerLoop(epfd int, events [1]unix.EpollEvent) {
	for {
		n, err := unix.EpollWait(epfd, events[:], -1)
		if err != nil {
			if e, ok := err.(syscall.Errno); ok {
				if e.Temporary() {
					continue
				}
			}
		}

		if n == 1 && events[0].Events&unix.EPOLLIN == unix.EPOLLIN {
			// we don't check, it has to be poweroff
			shutdown(syscall.LINUX_REBOOT_CMD_POWER_OFF)
		}
	}
}

func listenToPowerEventFile(name string) {

	var event unix.EpollEvent
	var events [1]unix.EpollEvent

	pwr, err := os.Open(fmt.Sprintf("/dev/input/%s", name))
	if err != nil {
		logWarn("can not listen to power off (dev): %s", err.Error())
		return
	}
	defer pwr.Close()
	fd := int(pwr.Fd())

	epfd, err := unix.EpollCreate1(0)
	if err != nil {
		logWarn("can not listen to power off (epoll): %s", err.Error())
	}
	defer unix.Close(epfd)

	event.Events = unix.EPOLLIN
	event.Fd = int32(fd)
	if err := unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, fd, &event); err != nil {
		logWarn("can not listen to power off (epolladd): %s", err.Error())
	}

	listenPowerLoop(epfd, events)

}

func prepSbinPower() {
	// create /sbin/poweroff, /sbin/reboot
	if _, err := os.Stat("/sbin"); os.IsNotExist(err) {
		err := os.Mkdir("/sbin", 0755)
		if err != nil {
			logWarn("can not listen to power off (orderly shutdown): %s", err.Error())
			return
		}
	}

	sbin := func(name string) {
		// create
		logDebug("removing %s", name)
		os.Remove(name)
		err := os.Symlink("/vorteil/vinitd", name)
		if err != nil {
			logWarn("can not listen to power off (shudown): %s", err.Error())
			return
		}
	}

	sbin("/sbin/poweroff")
	sbin("/sbin/reboot")

}

func listenToPowerEvent() {

	f, err := os.OpenFile("/proc/bus/input/devices", os.O_RDONLY, os.ModePerm)
	if err != nil {
		logWarn("can not listen to power off (devices): %s", err.Error())
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	toggle := false

	// find the handler string for device
	var l string
	for sc.Scan() {
		l = sc.Text()

		if strings.Contains(l, "Power Button") {
			toggle = true
		}

		if toggle && strings.Contains(l, "Handlers") {
			break
		}

	}

	if err := sc.Err(); err != nil {
		logWarn("can not listen to power off (scan): %s", err.Error())
		return
	}

	ss := strings.SplitN(l, "Handlers=", 2)

	if len(ss) != 2 {
		logWarn("can not listen to power off (handlers): no handler")
		return
	}

	for _, s := range strings.Fields(ss[1]) {
		if strings.HasPrefix(s, "event") {
			listenToPowerEventFile(s)
		}
	}

}
