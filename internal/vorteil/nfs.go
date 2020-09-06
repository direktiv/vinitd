package vorteil

import (
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/vorteil/vorteil/pkg/vcfg"
)

func resolveNFS(name string) net.IP {
	logDebug("resolving nfs server: %s", name)
	ips, err := net.LookupIP(name)
	if err != nil {
		logAlways("can not resolve %s", name)
		return nil
	}
	for _, i := range ips {
		if i.To4() != nil {
			return i
		}
	}
	return nil
}

func setupNFS(mounts []vcfg.NFSSettings) {

	for _, m := range mounts {

		srv := m.Server
		mp := m.MountPoint
		attrs := m.Arguments

		// split it at : to check if it is a server name or ip
		// format is servernam:mountpoint, e.g. myserver:/tmp
		srvInfo := strings.SplitN(srv, ":", 2)
		if len(srvInfo) != 2 {
			logError("can not parse nfs server %s, format server:mount", srv)
			continue
		}

		s := net.ParseIP(srvInfo[0])

		// we need to resolve the name. it is no ip
		if s == nil {
			s = resolveNFS(srvInfo[0])
		}

		if s == nil {
			logError("can not resolve %s", srvInfo[0])
			continue
		}

		var a []string
		a = append(a, attrs)
		a = append(a, fmt.Sprintf("addr=%s", s.String()))

		logAlways("nfs mount %s to %s with %s", srvInfo[1], mp, strings.Join(a[:], ","))
		os.MkdirAll(mp, 0755)

		err := syscall.Mount(fmt.Sprintf(":%s", srvInfo[1]), mp, "nfs", 0, strings.Join(a[:], ","))
		if err != nil {
			logError("can not mount NFS: %s", err.Error())
			continue
		}
	}

}
