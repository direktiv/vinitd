package vorteil

// func TestDNSUpAndCount(t *testing.T) {
//
// 	v := New(LogFnStdout)
//
// 	err := v.startDNS("127.0.0.1:5353")
// 	assert.NoError(t, err)
//
// 	assert.Empty(t, v.dns)
//
// 	dnsproxy.Close()
//
// 	v.dns = append(v.dns, net.ParseIP("8.8.8.8"), net.ParseIP("8.8.4.4"))
//
// 	err = v.startDNS("127.0.0.1:5354")
// 	assert.NoError(t, err)
//
// 	assert.Len(t, v.dns, 2)
//
// 	r := &net.Resolver{
// 		PreferGo: true,
// 		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
// 			d := net.Dialer{
// 				Timeout: time.Millisecond * time.Duration(10000),
// 			}
// 			return d.DialContext(ctx, "udp", "127.0.0.1:5354")
// 		},
// 	}
// 	ip, err := r.LookupHost(context.Background(), "www.google.com")
// 	assert.NoError(t, err)
//
// 	assert.NotEmpty(t, ip)
//
// 	dnsproxy.Close()
//
// }
