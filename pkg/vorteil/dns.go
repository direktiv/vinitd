/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/AdguardTeam/dnsproxy/upstream"
)

const (
	defaultDNSAddr = "127.0.0.1"
)

func printDNS(dns []string) {
	if len(dns) > 0 {
		logAlways("dns\t\t: %s", strings.Join(dns, ", "))
	} else {
		logAlways("dns\t\t: none")
	}
}

var dns []string

func (v *Vinitd) startDNS(dnsAddr string, verbose bool) error {

	// only add config DNS if not provided by DHCP
	if len(v.dns) == 0 {
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

	config := proxy.Config{
		Ratelimit:              0,
		CacheEnabled:           true,
		CacheSizeBytes:         65536,
		CacheMinTTL:            60,
		CacheMaxTTL:            600,
		RefuseAny:              false,
		EnableEDNSClientSubnet: true,
		UDPBufferSize:          65536,
		MaxGoroutines:          10,
		UpstreamMode:           proxy.UModeParallel,
	}

	ua := &net.UDPAddr{Port: 53, IP: net.ParseIP(defaultDNSAddr)}
	config.UDPListenAddr = append(config.UDPListenAddr, ua)

	ta := &net.TCPAddr{Port: 53, IP: net.ParseIP(defaultDNSAddr)}
	config.TCPListenAddr = append(config.TCPListenAddr, ta)

	upstreamConfig, err := proxy.ParseUpstreamsConfig(dns,
		upstream.Options{
			InsecureSkipVerify: false,
			Bootstrap:          []string{},
			Timeout:            10 * time.Second,
		})

	// upstreamConfig, err := proxy.ParseUpstreamsConfig(dns, []string{}, 10*time.Second)
	if err != nil {
		logError("can not start dns: %v", err)
		return err
	}
	config.UpstreamConfig = &upstreamConfig

	dnsProxy := proxy.Proxy{Config: config}
	err = dnsProxy.Start()

	return err
}
