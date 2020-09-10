package vorteil

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNullString(t *testing.T) {
	b := []byte{118, 111, 114, 116, 101, 105, 108, 0, 0, 0, 0, 0, 0, 0}
	str1 := "vorteil"
	str2 := terminatedNullString(b)

	assert.NotEqual(t, len(b), len([]byte(str2)))
	assert.Equal(t, len([]byte(str2)), len([]byte(str1)))
}

func TestUptime(t *testing.T) {
	up := uptime()
	assert.NotEqual(t, up, 0.0)
}

func TestUniqueIP(t *testing.T) {

	ip1 := []net.IP{net.ParseIP("192.168.1.1"),
		net.ParseIP("192.168.1.2"), net.ParseIP("192.168.1.3")}

	assert.Equal(t, len(ip1), len(uniqueIPs(ip1)))

	ip1 = append(ip1, net.ParseIP("192.168.1.1"))

	assert.NotEqual(t, len(ip1), len(uniqueIPs(ip1)))

}

func TestMin(t *testing.T) {
	assert.Equal(t, min(1, 2), uint32(1))
}

func TestNetworkInt2IP(t *testing.T) {
	// 0101a8c0
	// ipInt := 16885952
	//
	// // ip1 := networkInt2IP(uint32(ipInt))
	// ip2 := net.ParseIP("192.168.1.1")
	//
	// assert.Equal(t, ip1.String(), ip2.String())

}

func TestIP2networkInt(t *testing.T) {

	ipInt1 := 16885952

	ip1 := net.ParseIP("192.168.1.1")
	ipInt2 := ip2networkInt(ip1)

	assert.Equal(t, ipInt1, int(ipInt2))

}
