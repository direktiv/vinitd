package vorteil

import (
	"net"
	"strings"

	"github.com/Asphaltt/dnsproxy-go"
)

const (
	defaultDNSAddr = "127.0.0.1:53"
)

func (v *Vinitd) startDNS(dnsAddr string) error {

	var dns []string

	for _, d := range v.vcfg.System.DNS {
		ip := net.ParseIP(d)
		if ip != nil {
			v.dns = append(v.dns, ip)
		}
	}

	// remove potential duplicates
	v.dns = uniqueIPs(v.dns)

	// additional loop to add a dns from dhcp if there were any
	for _, d := range v.dns {
		dns = append(dns, d.String())
	}

	if len(dns) > 0 {
		logAlways("dns\t\t: %s", strings.Join(dns, ", "))
	} else {
		logAlways("dns\t\t: none")
		return nil
	}

	cfg := &dnsproxy.Config{
		Addr:          dnsAddr,
		UpServers:     dns,
		WithCache:     true,
		WorkerPoolMin: 5,
		WorkerPoolMax: 50,
	}

	err := dnsproxy.Start(cfg)
	if err != nil {
		return err
	}

	return nil
}
