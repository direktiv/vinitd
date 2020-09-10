package vorteil

import (
	"encoding/binary"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
)

const (
	procFile = "/proc/uptime"
)

func uniqueIPs(ipSlice []net.IP) []net.IP {
	keys := make(map[string]bool)
	list := []net.IP{}
	for _, entry := range ipSlice {
		if _, value := keys[entry.String()]; !value {
			keys[entry.String()] = true
			list = append(list, entry)
		}
	}
	return list
}

func min(x, y uint32) uint32 {
	if x < y {
		return x
	}
	return y
}

func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

// func networkInt2IP(n uint32) net.IP {
// 	ip := make(net.IP, 4)
// 	binary.LittleEndian.PutUint32(ip, n)
// 	return ip
// }
//
func ip2networkInt(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.LittleEndian.Uint32(ip[12:16])
	}
	return binary.LittleEndian.Uint32(ip)
}

func terminatedNullString(in []byte) string {

	var target []byte

	for _, c := range in {
		if c == 0 {
			break
		}
		target = append(target, c)
	}

	return string(target)

}

func uptime() float64 {

	up, err := ioutil.ReadFile(procFile)
	if err != nil {
		return 0.0
	}

	// uptime has two values
	// the first one is the uptime in seconds
	f, err := strconv.ParseFloat(strings.SplitN(string(up), " ", 2)[0], 8)
	if err != nil {
		return 0.0
	}

	return f
}
