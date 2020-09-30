/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	ext4IOCResizeFS = 0x40086610
	xfsGrowFS       = 0x4010586e
	xfsGeom         = 0x8100587e
)

type xfsGrowFSData struct {
	newBlocks uint64
	imaxpct   uint32
}

type xfsFsopGeom struct {
	blockSize    uint32
	rtExtSize    uint32
	agBlocks     uint32
	agCount      uint32
	logBlocks    uint32
	sectSize     uint32
	inodeSize    uint32
	iMaxPct      uint32
	dataBlocks   uint64
	rtBlocks     uint64
	rtExtents    uint64
	logStart     uint64
	uuid         [16]byte
	sUnit        uint32
	sWidth       int32
	version      uint32
	flags        uint32
	logSectSize  uint32
	rtSectSize   uint32
	dirBlockSize uint32
	logsUnit     uint32
}

func flushDisk(p string) {

	f, err := os.Open(p)
	if err != nil {
		return
	}

	unix.IoctlRetInt(int(f.Fd()), unix.BLKFLSBUF)
	unix.Syscall(unix.SYS_SYNCFS, f.Fd(), 0, 0)

	f.Close()

	syscall.Sync()
}

func mountFs(target, fstype, options string) error {

	if _, err := os.Stat(target); os.IsNotExist(err) {
		if target == "/proc" || target == "/sys" {
			SystemPanic(fmt.Sprintf("file '%s' does not exist", target))
		}
		err := os.MkdirAll(target, 0755)
		if err != nil {
			return err
		}
	}

	return syscall.Mount("none", target, fstype, 0, options)
}

func changeDiskScheduler(vdisk string) {

	// it is always /dev/DISKNAME so this is safe
	disk := strings.SplitN(vdisk, "/", 3)[2]
	ioutil.WriteFile(fmt.Sprintf("/sys/block/%s/queue/scheduler", disk), []byte("noop"), 0644)

}

// create and mount basic structure. arg for base firectory for testing only
func setupBasicDirectories(base string) error {

	bd := func(n string) string {
		return filepath.Join(base, n)
	}

	os.Chmod(bd("/tmp"), 0777)

	type dir struct {
		path, fstype string
	}

	dirs := []dir{
		{bd("/proc"), "proc"},
		{bd("/sys"), "sysfs"},
		{bd("/dev/pts"), "devpts"},
	}

	for _, d := range dirs {
		err := mountFs(d.path, d.fstype, "")
		if err != nil {
			return err
		}
	}

	os.Symlink("/proc/self/fd", "/dev/fd")

	return nil
}

func growDisks() error {

	p, err := bootDisk()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(p, os.O_RDWR|os.O_SYNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	err = growDisk(f, p)
	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	flushDisk(p)
	return nil
}

func growDisk(f *os.File, p string) error {

	gptGrower, err := newGPTModifier(&newGPTModifierArgs{
		File: f,
		Path: p,
	})
	if err != nil {
		return err
	}

	if !gptGrower.needsResize() {
		return nil
	}

	err = gptGrower.grow()
	if err != nil {
		return err
	}

	// if reboot {
	rootFS, err := os.Open("/")
	if err != nil {
		return err
	}
	defer rootFS.Close()

	// get blocksize
	s1 := syscall.Statfs_t{}
	err = syscall.Fstatfs(int(rootFS.Fd()), &s1)
	if err != nil {
		return err
	}

	blocks := (gptGrower.partitionEntry.LastLBA - gptGrower.partitionEntry.FirstLBA) * sectorSize / uint64(s1.Bsize)

	arg := &unix.BlkpgIoctlArg{
		Op: unix.BLKPG_RESIZE_PARTITION,
		Data: (*byte)(unsafe.Pointer(&unix.BlkpgPartition{
			Start:  int64(gptGrower.partitionEntry.FirstLBA * sectorSize),                                      // in bytes
			Length: int64((gptGrower.partitionEntry.LastLBA - gptGrower.partitionEntry.FirstLBA) * sectorSize), // in bytes
			Pno:    int32(2),
		})),
	}

	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(f.Fd()), unix.BLKPG, uintptr(unsafe.Pointer(arg))); e != 0 {
		return fmt.Errorf("error resizing gpt: %s", syscall.Errno(e))
	}

	// detect fs type
	format, err := detectFormat(f, p, int64(gptGrower.partitionEntry.FirstLBA*sectorSize))
	if err != nil {
		return err
	}

	switch format {
	case XFS:
		logDebug("detected xfs filesystem")
		x := new(xfsGrowFSData)

		geom := new(xfsFsopGeom)
		if _, _, e := syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(rootFS.Fd()),
			uintptr(xfsGeom),
			uintptr(unsafe.Pointer(geom)),
		); e != 0 {
			return fmt.Errorf("error getting xfs geometry: %s", syscall.Errno(e))
		}

		x.newBlocks = blocks
		x.imaxpct = geom.iMaxPct

		if _, _, e := syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(rootFS.Fd()),
			uintptr(xfsGrowFS),
			uintptr(unsafe.Pointer(x)),
		); e != 0 {
			return fmt.Errorf("error resizing xfs filesystem: %s", syscall.Errno(e))
		}

	case Ext2FS:
		fallthrough
	case Ext4FS:
		logDebug("detected ext filesystem")
		resizeFSFlag := uintptr(ext4IOCResizeFS)
		if _, _, e := syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(rootFS.Fd()),
			resizeFSFlag,
			uintptr(unsafe.Pointer(&blocks)),
		); e != 0 {
			return fmt.Errorf("error resizing ext filesystem: %s", syscall.Errno(e))
		}
	}

	return nil
}
