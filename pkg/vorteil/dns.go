/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"fmt"
	"net"
	"strings"

	"github.com/Asphaltt/dnsproxy-go"
)

const (
	defaultDNSAddr = "127.0.0.1:53"
)

func printDNS(dns []string) {
	if len(dns) > 0 {
		logAlways("dns\t\t: %s", strings.Join(dns, ", "))
	} else {
		logAlways("dns\t\t: none")
	}
}

func (v *Vinitd) startDNS(dnsAddr string, verbose bool) error {

	var dns []string

	for _, d := range v.vcfg.System.DNS {

		// replace envs
		for k, val := range v.hypervisorInfo.envs {
			d = strings.ReplaceAll(d, fmt.Sprintf(replaceString, k), val)
		}

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

	if verbose {
		printDNS(dns)
	}

	// don't start
	if len(dns) == 0 {
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
