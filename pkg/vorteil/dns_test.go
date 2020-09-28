package vorteil

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReadVCFGFile1(t *testing.T) {

	v := New(testLogFn)
	v.readVCFG("/hw.raw")

	// empty so should return
	err := v.startDNS(defaultDNSAddr, false)
	assert.NoError(t, err)

	// already running
	v.vcfg.System.DNS = append(v.vcfg.System.DNS, "8.8.4.4")
	err = v.startDNS(defaultDNSAddr, false)
	assert.Error(t, err)

	err = v.startDNS("127.0.0.1:63", false)
	assert.NoError(t, err)

	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, "udp", "127.0.0.1:63")
		},
	}

	ip, err := r.LookupHost(context.Background(), "www.google.com")
	assert.NoError(t, err)
	assert.NotNil(t, ip)

}
