/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	easy "github.com/t-tomalak/logrus-easy-formatter"
	"github.com/vorteil/vorteil/pkg/vcfg"
)

var (
	logger *logrus.Logger
)

func logAlways(format string, values ...interface{}) {
	txt := fmt.Sprintf(format, values...)
	up := fmt.Sprintf("[%05.6f]", uptime())
	fmt.Fprintf(os.Stdout, "%s %s\n", up, txt)
}

func logDebug(format string, values ...interface{}) {
	logger.Debugf(fmt.Sprintf(format+"\n", values...))
}

// LogDebugEarly creates an early debug logging function before logging is configured.
// Used from the command
func LogDebugEarly(format string, values ...interface{}) {

	txt := fmt.Sprintf("<%d>%s", 7, fmt.Sprintf(format, values...))
	f, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	_, err = f.Write([]byte(txt))
	if err != nil {
		return
	}

}

func logWarn(format string, values ...interface{}) {
	logger.Warnf(fmt.Sprintf(format+"\n", values...))
}

// SystemPanic prints error message and shuts down the system
func SystemPanic(format string, values ...interface{}) {
	logger.Errorf(fmt.Sprintf(format, values...))
	shutdown(syscall.LINUX_REBOOT_CMD_POWER_OFF)
}

func logError(format string, values ...interface{}) {
	logger.Errorf(fmt.Sprintf(format+"\n", values...))
}

func printVersion() error {

	pv, err := ioutil.ReadFile("/proc/version")
	if err != nil {
		return err
	}

	s := strings.Split(string(pv), "(")
	version := s[0]

	kv, err := ioutil.ReadFile("/proc/sys/kernel/version")
	if err != nil {
		return err
	}

	logAlways("%s (%s)", strings.TrimSpace(string(kv)), strings.TrimSpace(version))

	return nil
}

func enableTTYS() (*os.File, error) {
	return os.OpenFile("/dev/ttyS0", os.O_RDWR, 0755)
}

func enableTTY() (*os.File, error) {
	return os.OpenFile("/dev/tty1", os.O_RDWR, 0755)
}

func enableTTYRedir() (*os.File, error) {
	return os.OpenFile("/vlogs/stdout", os.O_RDWR, 0755)
}

func (v *Vinitd) setupVtty(mode vcfg.StdoutMode) {

	var err error

	switch mode {
	case vcfg.StdoutModeScreenOnly:
		v.tty, err = enableTTY()
		if err != nil {
			logAlways("can not setup tty: %v", err)
		}
		v.ttyS = nil
	case vcfg.StdoutModeSerialOnly:
		v.ttyS, err = enableTTYS()
		if err != nil {
			logAlways("can not setup ttys: %v", err)
		}
		v.tty = nil
	case vcfg.StdoutModeDisabled:
		v.tty = nil
		v.ttyS = nil
	default:
		v.ttyS, err = enableTTYS()
		if err != nil {
			logAlways("can not setup ttys: %v", err)
		}
		v.tty, err = enableTTY()
		if err != nil {
			logAlways("can not setup tty: %v", err)
		}
	}

	os.Stdout, err = os.OpenFile(defaultTTY,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logWarn("can not assign /dev/vtty to vinitd")
	}

	os.Stderr, err = os.OpenFile(defaultTTY,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logWarn("can not assign /dev/vtty to vinitd")
	}

}

func init() {

	f, err := os.OpenFile(defaultTTY, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		logAlways("can not assign /dev/vtty to logger")
		return
	}

	logger = &logrus.Logger{
		Out:   f,
		Level: logrus.ErrorLevel,
		Formatter: &easy.Formatter{
			TimestampFormat: "01-02 15:04:05",
			LogFormat:       "[%lvl%]: %time% - %msg%",
		},
	}
}

func setLoglevel() {

	var (
		i   int
		err error
		b   []byte
	)

	// setup log level
	b, err = ioutil.ReadFile("/proc/sys/kernel/printk")
	if err != nil {
		logAlways("can not configure logging: %v", err)
		return
	}

	i, err = strconv.Atoi(string(b[0]))
	if err != nil {
		logAlways("can not get log level: %v", err)
		i = 1
	}

	if i > 6 {
		logger.Level = logrus.DebugLevel
	} else if i > 3 {
		logger.Level = logrus.WarnLevel
	} else {
		logger.Level = logrus.ErrorLevel
	}

}

func (v *Vinitd) checkLogs() {

	setLoglevel()

	var event syscall.EpollEvent
	var events [32]syscall.EpollEvent

	file, err := os.Open(defaultTTY)
	if err != nil {
		logAlways("can not setup logging: %v", err)
		return
	}

	fd := int(file.Fd())
	if err := syscall.SetNonblock(fd, true); err != nil {
		logAlways("can not setup logging: %v", err)
		return
	}

	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		logAlways("can not setup logging: %v", err)
		return
	}

	defer syscall.Close(epfd)
	event.Events = syscall.EPOLLIN
	event.Fd = int32(fd)

	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &event); err != nil {
		logAlways("can not setup logging: %v", err)
		return
	}

	b1 := make([]byte, 65536)

	for {

		nevents, err := syscall.EpollWait(epfd, events[:], -1)

		if err != nil {
			if e, ok := err.(syscall.Errno); ok {
				if e.Temporary() {
					continue
				}
			}
			logError("epoll error: %v", err)
			continue
		}

		ts := []*os.File{v.tty, v.ttyS, v.ttyRedir}

		for ev := 0; ev < nevents; ev++ {
			r, err := file.Read(b1)

			if err != nil {
				logError("epoll error: %v", err)
			}

			for _, t := range ts {
				if t != nil {
					t.Write(b1[:r])
				}
			}
		}

	}

}
