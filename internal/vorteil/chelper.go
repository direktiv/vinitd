/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

// #cgo CFLAGS: -g -Wall
// #include "helper.h"
// #include "vmtools.h"
// #include <stdlib.h>
import "C"
import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

//export RebootForTools
func RebootForTools() {
	shutdown(syscall.LINUX_REBOOT_CMD_RESTART, 0)
}

//export ShutdownForTools
func ShutdownForTools() {
	shutdown(syscall.LINUX_REBOOT_CMD_POWER_OFF, 0)
}

//export UptimeForTools
func UptimeForTools() C.int {
	var d1, d2 uint64
	up := uptime()
	fmt.Sscanf(fmt.Sprintf("%f", up), "%d.%d", &d1, &d2)
	return C.int(d1*100 + d2)
}

func startVMTools(cards int, hostname string) {
	hn := C.CString(hostname)
	defer C.free(unsafe.Pointer(hn))
	C.vmtools_start(C.int(cards), hn)
}

func addNetworkRoute4(dst, mask, gw net.IP, dev string, flags int) error {

	var dstNwOrder, maskNwOrder, gwNwOrder int

	direct := C.CString(dev)
	defer C.free(unsafe.Pointer(direct))

	nw := func(v net.IP) int {
		if v != nil {
			return int(ip2networkInt(v))
		}
		return 0
	}

	dstNwOrder = nw(dst)
	maskNwOrder = nw(mask)
	gwNwOrder = nw(gw)

	err := C.helper_add_route(C.int(dstNwOrder),
		C.int(maskNwOrder), C.int(gwNwOrder), direct, C.int(flags))

	if err != 0 {
		return fmt.Errorf("could not set route for %s", dev)
	}

	return nil

}
