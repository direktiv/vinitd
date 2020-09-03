package vorteil

const (
	defaultDnsAddr = "127.0.0.1:53"
)

func (v *Vinitd) startDNS(dnsAddr string) error {

	// TODO: DNS
	// var dns []string
	//
	// for _, d := range v.vcfg.Net.DNS {
	// 	if d == 0 {
	// 		break
	// 	}
	// 	v.dns = append(v.dns, networkInt2IP(d))
	// }
	//
	// // remove potential duplicates
	// v.dns = uniqueIPs(v.dns)
	//
	// // additional loop to add a dns from dhcp if there were any
	// for _, d := range v.dns {
	// 	dns = append(dns, d.String())
	// }
	//
	// if len(dns) > 0 {
	// 	logAlways("dns\t\t: %s", strings.Join(dns, ", "))
	// } else {
	// 	logAlways("dns\t\t: none")
	// 	return nil
	// }
	//
	// cfg := &dnsproxy.Config{
	// 	Addr:          dnsAddr,
	// 	UpServers:     dns,
	// 	WithCache:     true,
	// 	WorkerPoolMin: 5,
	// 	WorkerPoolMax: 50,
	// }
	//
	// err := dnsproxy.Start(cfg)
	// if err != nil {
	// 	return err
	// }

	return nil
}
