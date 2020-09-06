package vorteil

import (
	"bufio"
	"encoding/binary"
	"log"
	"os/exec"
	"runtime"
	"sort"
	"time"
	"unsafe"

	"fmt"
	"io/ioutil"
	mrand "math/rand"
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/client4"
	"github.com/vishvananda/netlink"
	"github.com/vorteil/vorteil/pkg/vcfg"
	"golang.org/x/sys/unix"
)

type networkType int

const (
	DEVTYPE_UNKOWN    networkType = iota
	DEVTYPE_NET                   = iota
	DEVTYPE_LOCALHOST             = iota
)

const (
	deviceNet                 = "1"
	deviceLocal               = "772"
	dhcpAttempts              = 3
	attemptLoops              = 10
	dhcpDefaultRenew          = 360
	azureEndpointServerOption = 245

	ETHTOOL_SGSO       = 0x00000024
	ETHTOOL_SUFO       = 0x00000022
	ETHTOOL_STSO       = 0x0000001f
	ETHTOOL_SRXCSUM    = 0x00000015
	ETHTOOL_STXCSUM    = 0x00000017
	ETHTOOL_SSG        = 0x00000019
	ETHTOOL_GCHANNELS  = 0x0000003c
	ETHTOOL_SCHANNELS  = 0x0000003d
	ETHTOOL_GRINGPARAM = 0x00000010
	ETHTOOL_SRINGPARAM = 0x00000011
	SIOCETHTOOL        = 0x8946
	IFNAMSIZ           = 16
)

var (
	tsoAttrs = []int{ETHTOOL_SSG, ETHTOOL_SUFO, ETHTOOL_STSO,
		ETHTOOL_SGSO, ETHTOOL_SRXCSUM, ETHTOOL_STXCSUM}
	HOSTNAME_SALT = "$SALT" // replace with 8 random chars
)

// TCPDUMP vars
var (
	tcpdumpSnapshotLen int32         = 1024
	tcpdumpPromiscuous bool          = false
	tcpdumpBPFFilter   string        = "tcp or udp"
	tcpdumpTimeout     time.Duration = 10 * time.Second
)

type ifreq struct {
	ifr_name [IFNAMSIZ]byte
	ifr_data uintptr
}

type ethtoolValue struct {
	cmd  uint32
	data uint32
}

type channels struct {
	cmd           uint32
	maxRx         uint32
	maxTx         uint32
	maxOther      uint32
	maxCombined   uint32
	rxCount       uint32
	txCount       uint32
	otherCount    uint32
	combinedCount uint32
}

type ringparam struct {
	cmd                  uint32
	rx_max_pending       uint32
	rx_mini_max_pending  uint32
	rx_jumbo_max_pending uint32
	tx_max_pending       uint32
	rx_pending           uint32
	rx_mini_pending      uint32
	rx_jumbo_pending     uint32
	tx_pending           uint32
}

func networkDeviceType(name string) networkType {
	dat, err := ioutil.ReadFile(fmt.Sprintf("/sys/class/net/%s/type", name))
	if err != nil {
		return DEVTYPE_UNKOWN
	}

	s := strings.TrimSpace(string(dat))
	switch s {
	case deviceNet:
		return DEVTYPE_NET
	case deviceLocal:
		return DEVTYPE_LOCALHOST
	default:
		return DEVTYPE_UNKOWN
	}

}

func ioctl(ifc string, data uintptr) error {

	var name [IFNAMSIZ]byte
	copy(name[:], []byte(ifc))

	ifr := ifreq{
		ifr_name: name,
		ifr_data: data,
	}

	fd, _ := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_IP)
	defer unix.Close(fd)

	_, _, ep := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), SIOCETHTOOL,
		uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return ep
	}

	return nil
}

func checkRingParams(ringparam ringparam) bool {

	needsUpdate := false

	if ringparam.rx_max_pending > ringparam.rx_pending {
		needsUpdate = true
		ringparam.rx_pending = ringparam.rx_max_pending
	}

	if ringparam.tx_max_pending > ringparam.tx_pending {
		needsUpdate = true
		ringparam.tx_pending = ringparam.tx_max_pending
	}

	return needsUpdate

}

func checkChannels(channels channels) bool {

	needsUpdate := false

	// do not use more channels than cpus
	cpus := uint32(runtime.NumCPU())

	if min(cpus, channels.maxCombined) > channels.combinedCount {
		needsUpdate = true
		channels.combinedCount = channels.maxCombined
	}

	if min(cpus, channels.maxTx) > channels.txCount {
		needsUpdate = true
		channels.txCount = channels.maxTx
	}

	if min(cpus, channels.maxRx) > channels.rxCount {
		needsUpdate = true
		channels.rxCount = channels.maxRx
	}

	return needsUpdate
}

func configQueues(ifcs map[string]*ifc) {

	for _, ifc := range ifcs {

		// needsUpdate := false
		channels := channels{
			cmd: ETHTOOL_GCHANNELS,
		}

		if err := ioctl(ifc.name, uintptr(unsafe.Pointer(&channels))); err != nil {
			goto ringconfig
		}

		// if update required we send one. errors can not be handled
		if checkChannels(channels) {
			logDebug("updating network queues")
			channels.cmd = ETHTOOL_SCHANNELS
			ioctl(ifc.name, uintptr(unsafe.Pointer(&channels)))
		}

	ringconfig:
		ringparam := ringparam{
			cmd: ETHTOOL_GRINGPARAM,
		}

		if err := ioctl(ifc.name, uintptr(unsafe.Pointer(&ringparam))); err != nil {
			return
		}

		if checkRingParams(ringparam) {
			logDebug("updating network ringparams")
			ringparam.cmd = ETHTOOL_SRINGPARAM
			ioctl(ifc.name, uintptr(unsafe.Pointer(&ringparam)))
		}

	}
}

func renewDHCP(name string, client *client4.Client,
	offer *dhcpv4.DHCPv4, cid []byte, xid dhcpv4.TransactionID) (*dhcpv4.DHCPv4, error) {

	rfd, err := client4.MakeListeningSocket(name)
	if err != nil {
		return nil, err
	}

	sfd, err := client4.MakeBroadcastSocket(name)
	if err != nil {
		return nil, err
	}

	defer closeFds(sfd, rfd)

	request, err := dhcpv4.NewRequestFromOffer(offer,
		dhcpv4.WithTransactionID(xid),
		dhcpv4.WithOption(dhcpv4.OptClientIdentifier(cid)),
		dhcpv4.WithBroadcast(true),
		dhcpv4.WithRequestedOptions(dhcpv4.OptionRenewTimeValue, dhcpv4.OptionNTPServers,
			dhcpv4.GenericOptionCode(azureEndpointServerOption)))

	if err != nil {
		return nil, err
	}

	ack, err := client.SendReceive(sfd, rfd, request, dhcpv4.MessageTypeAck)
	if err != nil {
		return nil, err
	}

	return ack, nil
}

func closeFds(sfd, rfd int) {
	if err := unix.Close(sfd); err != nil {
		log.Printf("unix.Close(sendFd) failed: %v", err)
	}
	if sfd != rfd {
		if err := unix.Close(rfd); err != nil {
			log.Printf("unix.Close(recvFd) failed: %v", err)
		}
	}
}

func addAddrToInterface(ifc *ifc) error {

	eth, err := netlink.LinkByName(ifc.name)
	if err != nil {
		return err
	}

	ipConfig := &netlink.Addr{IPNet: ifc.addr}

	if err = netlink.AddrAdd(eth, ipConfig); err != nil {
		return err
	}

	return nil
}

func dhcpDiscover(ifc net.Interface,
	clientID []byte) (*dhcpv4.DHCPv4, dhcpv4.TransactionID, error) {

	var (
		err             error
		offer, discover *dhcpv4.DHCPv4
		rfd, sfd        int
		xid             dhcpv4.TransactionID
	)

	for i := 0; i < attemptLoops; i++ {

		logDebug("discover request for %s", ifc.Name)
		sfd, err = client4.MakeBroadcastSocket(ifc.Name)
		if err != nil {
			return nil, xid, err
		}

		rfd, err = client4.MakeListeningSocket(ifc.Name)
		if err != nil {
			return nil, xid, err
		}

		defer closeFds(sfd, rfd)
		c := client4.NewClient()
		c.ReadTimeout = 10 * time.Second
		c.WriteTimeout = 10 * time.Second

		for a := 0; a < dhcpAttempts; a++ {
			mrand.Read(xid[:])
			discover, _ = dhcpv4.NewDiscoveryForInterface(ifc.Name,
				dhcpv4.WithTransactionID(xid),
				dhcpv4.WithOption(dhcpv4.OptClientIdentifier(clientID)),
				dhcpv4.WithBroadcast(true),
				dhcpv4.WithRequestedOptions(dhcpv4.OptionRenewTimeValue, dhcpv4.OptionNTPServers,
					dhcpv4.GenericOptionCode(azureEndpointServerOption)))

			offer, err = c.SendReceive(sfd, rfd, discover,
				dhcpv4.MessageTypeOffer)

			if offer != nil {
				return offer, xid, err
			}
		}

		logWarn("can not get dhcp ip: %v, try %d", err, i)
		closeFds(sfd, rfd)

	}

	return nil, xid, err

}

func configInterface(ifc *ifc, ip, mask, router net.IP) error {

	logDebug("%s: %v/%v/%v", ifc.name, ip.String(), mask, router)

	ifc.addr = &net.IPNet{
		IP:   ip,
		Mask: net.IPMask(mask),
	}
	ifc.gw = router

	// add addr to interface
	addAddrToInterface(ifc)

	// google cloud returns a full mask, need to set link to gateway
	// if that fails we can panic because there is no connectivity
	if mask.Equal(net.IPv4bcast) {
		err := addNetworkRoute4(router, net.IPv4bcast, nil, ifc.name, unix.RTF_UP|unix.RTF_HOST)
		if err != nil {
			SystemPanic("could not set host route")
		}
	}

	// set default gateway
	if router != nil {
		logDebug("setting default gateway to %s", router)
		err := setDefaultGateway(ifc.name, router)
		if err != nil {
			SystemPanic(err.Error())
		}
	}

	return nil

}

func fetchDHCP(ifc *ifc, v *Vinitd) error {

	client := client4.NewClient()

	cid := make([]byte, len(ifc.netIfc.HardwareAddr)+1)
	cid[0] = byte(1)
	copy(cid[1:], ifc.netIfc.HardwareAddr)

	offer, xid, err := dhcpDiscover(ifc.netIfc, cid)
	if err != nil {
		logError("can not get IP from DHCP: %s", err.Error())
		return err
	}

	// we are happy with the offer response
	// if ack is not successful we panic later
	router := dhcpv4.GetIP(dhcpv4.OptionRouter, offer.Options)
	mask := dhcpv4.GetIP(dhcpv4.OptionSubnetMask, offer.Options)
	dhcpServerIP := dhcpv4.GetIP(dhcpv4.OptionServerIdentifier, offer.Options)

	if len(offer.Options.Get(dhcpv4.GenericOptionCode(azureEndpointServerOption))) > 0 {
		v.hypervisorInfo.cloud = CP_AZURE
	} else {
		v.hypervisorInfo.cloud = CP_NONE
	}

	configInterface(ifc, offer.YourIPAddr, mask, router)

	renew := dhcpDefaultRenew
	rv := offer.Options.Get(dhcpv4.OptionRenewTimeValue)
	if rv != nil {
		renew = int(binary.BigEndian.Uint32(rv))
	}

	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", dhcpServerIP.String(), dhcpv4.ServerPort))
	if err != nil {
		logError("can not parse server address: %s", err.Error())
		return err
	}
	client.RemoteAddr = addr

	addr, err = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", offer.YourIPAddr.String(), dhcpv4.ClientPort))
	if err != nil {
		logError("can not parse client address: %s", err.Error())
		return err
	}
	client.LocalAddr = addr
	client.ReadTimeout = 10 * time.Second
	client.WriteTimeout = 10 * time.Second

	// add DNS
	v.dns = append(v.dns, offer.DNS()...)

	// TODO: ntp, at the moment we only use provided ntp servers
	// v.ntp = append(v.ntp, offer.NTPServers()...)

	go func(name string, client *client4.Client, offer *dhcpv4.DHCPv4) {

		offer.SetUnicast()

		// this is getting the ack,if not we panic because we are using that IP already
		ack, err := renewDHCP(name, client, offer, cid, xid)
		if err != nil {
			logWarn("can not ack IP address: %s", err.Error())
		}
		logDebug("dhcp acknowledged: %v", ack)

		for {
			<-time.After(time.Duration(renew) * time.Second)
			logDebug("renew with %v", dhcpServerIP)
			renewDHCP(name, client, offer, cid, xid)
		}

	}(ifc.name, client, offer)

	return nil

}

func setDefaultGateway(name string, ip net.IP) error {

	err := addNetworkRoute4(nil, nil, ip, name, unix.RTF_UP|unix.RTF_GATEWAY)

	if err != nil {
		return fmt.Errorf("could not set host route")
	}

	return nil

}

// configure network card offloading
func setTSOValues(name string, val byte) {

	logDebug("setting tso to %d", val)

	var nameIn [IFNAMSIZ]byte
	copy(nameIn[:], []byte(name))

	for _, attr := range tsoAttrs {

		cmd := ethtoolValue{
			cmd:  uint32(attr),
			data: uint32(val),
		}

		err := ioctl(name, uintptr(unsafe.Pointer(&cmd)))
		if err != nil {
			// not all network cards support it so we don't print to stderr
			logDebug("can not set tso to %d", val)
		}
	}

}

func startLink(name string) (netlink.Link, error) {

	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		return nil, err
	}

	return link, nil

}

func handleNetworkLink(interf *ifc, ifcg vcfg.NetworkInterface, v *Vinitd, errCh chan error, wg *sync.WaitGroup) {

	if ifcg.IP != "dhcp" {

		go func() {
			// static ip
			ip := net.ParseIP(ifcg.IP)
			mask := net.ParseIP(ifcg.Mask)
			gw := net.ParseIP(ifcg.Gateway)

			if ip == nil || mask == nil || gw == nil {
				errCh <- fmt.Errorf("ip, mask or gateway is not valid")
				wg.Done()
				return
			}
			configInterface(interf, ip, mask, gw)
			wg.Done()
		}()

	} else {

		go func(interf *ifc, v *Vinitd) {
			err := fetchDHCP(interf, v)
			if err != nil {
				errCh <- err
			}
			wg.Done()
		}(interf, v)

	}

}

// logFunc is getting passed here so it can be easier to test the output in the Go tests
func handleNetworkTCPDump(interf *ifc, ifcg vcfg.NetworkInterface,
	errCh chan error, wg *sync.WaitGroup) {
	if ifcg.TCPDUMP {
		deviceFlag := fmt.Sprintf("--device=%s", interf.name)

		// Create tcpdump command
		tcpDumpCmd := exec.Command("/vorteil/tcpdump", deviceFlag)
		tcpdumpReader, err := tcpDumpCmd.StdoutPipe()
		if err != nil {
			errCh <- fmt.Errorf("could not set tcpdump stdoutPipe, %v", err)
			wg.Done()
			return
		}

		// Create routine to print tcpdump output
		go func(scanner *bufio.Scanner) {
			for scanner.Scan() {
				if line := scanner.Text(); line != "" {
					logAlways(line)
				}
			}
		}(bufio.NewScanner(tcpdumpReader))

		// Start tcpdump command
		err = tcpDumpCmd.Start()
		if err != nil {
			errCh <- fmt.Errorf("could not set tcpdump command, %v", err)
		}
	}

	wg.Done()
}

func (v *Vinitd) NetworkSetup() error {

	ifaces, err := net.Interfaces()
	if err != nil {
		logError("can not get network interfaces: %s", err.Error())
		return err
	}

	// interface counter
	ic := 0

	var wg sync.WaitGroup
	errCh := make(chan error)
	doneCh := make(chan bool)

	for _, i := range ifaces {

		deviceType := networkDeviceType(i.Name)

		// only handle devices
		if deviceType < DEVTYPE_NET {
			continue
		}

		logDebug("configure %s", i.Name)

		link, err := startLink(i.Name)
		if err != nil {
			logError("can not get enable network device %s: %s", i.Name, err.Error())
			return err
		}

		if deviceType == DEVTYPE_LOCALHOST {
			ipnet := &net.IPNet{
				IP:   net.IPv4(127, 0, 0, 1),
				Mask: net.IPv4Mask(255, 0, 0, 0),
			}

			addr := &netlink.Addr{IPNet: ipnet}
			netlink.AddrAdd(link, addr)
			netlink.LinkSetMTU(link, 65536)
		} else {

			// add the device to the list
			ifName := fmt.Sprintf("eth%d", ic)
			v.ifcs[ifName] = &ifc{
				name:   ifName,
				idx:    ic,
				netIfc: i,
			}

			ifcg := v.vcfg.Networks[ic]

			logDebug("set mtu to %d for %s", ifcg.MTU, i.Name)
			netlink.LinkSetMTU(link, int(ifcg.MTU))

			logDebug("disable tso: %v", ifcg.DisableTCPSegmentationOffloading)
			if ifcg.DisableTCPSegmentationOffloading {
				setTSOValues(i.Name, 0)
			} else {
				setTSOValues(i.Name, 1)
			}
			wg.Add(2)
			handleNetworkTCPDump(v.ifcs[ifName], ifcg, errCh, &wg)
			handleNetworkLink(v.ifcs[ifName], ifcg, v, errCh, &wg)
			ic++
		}
	}

	// wait for network setup
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		break
	case err := <-errCh:
		close(errCh)
		return err
	}

	logDebug("network configured")

	// Sort interface keys for printing
	ifcKeys := make([]string, 0, len(v.ifcs))
	for k := range v.ifcs {
		ifcKeys = append(ifcKeys, k)
	}

	sort.Strings(ifcKeys)
	for _, iKey := range ifcKeys {
		logAlways("%s ip\t: %s", v.ifcs[iKey].name, v.ifcs[iKey].addr.IP.String())
		logAlways("%s mask\t: %s", v.ifcs[iKey].name, net.IP(v.ifcs[iKey].addr.Mask).String())
		logAlways("%s gateway\t: %s", v.ifcs[iKey].name, v.ifcs[iKey].gw.String())
	}

	if len(v.ifcs) == 0 {
		logAlways("ip\t: no network devices available")
	}

	configRoutes(v.vcfg.Routing)

	go configQueues(v.ifcs)

	return nil
}

func setHostname(str string) (string, error) {

	// substitute keys in hostname string
	if strings.Contains(str, HOSTNAME_SALT) {
		var runes = strings.Split("abcdefghijklmnopqrstuvwxyz-0123456789", "")

		// already seeded here
		var salt string
		for i := 0; i < 8; i++ {
			salt += runes[mrand.Int()%len(runes)]
		}
		str = strings.Replace(str, HOSTNAME_SALT, salt, -1)
	}

	// convert to lowercase (uppercase letters are not valid hostname characters)
	str = strings.ToLower(str)

	hh, err := validateHostname(str)
	if err != nil {
		return "", err
	}

	return hh, nil
}

func validateHostname(hostname string) (string, error) {

	var h string
	printfStr := "%s%s"

	if len(hostname) == 0 {
		return "", fmt.Errorf("hostname can not be empty")
	}

	elemRegex, err := regexp.Compile(`[a-z0-9-]`)
	if err != nil {
		return hostname, err
	}

	hostname = strings.TrimPrefix(hostname, "-")

	elements := strings.Split(hostname, ".")

	for k, e := range elements {
		if len(e) == 0 {
			continue
		}
		// must contains only legal characters (a-z0-9 and -)
		// replace illegal characters with '-'
		indexes := elemRegex.FindAllIndex([]byte(e), -1)
		var validatedElem string
		if k != 0 {
			validatedElem = "."
		}
		for i := 0; i < len(indexes); i++ {
			newElem := e[indexes[i][0]:indexes[i][1]]
			if i != 0 {
				endPrevMatch := indexes[i-1][1]
				if indexes[i][0]-endPrevMatch > 0 {
					validatedElem = fmt.Sprintf(printfStr, validatedElem, strings.Repeat("-", indexes[i][0]-endPrevMatch))
				}
			}
			validatedElem = fmt.Sprintf(printfStr, validatedElem, newElem)
		}

		validatedElem = fmt.Sprintf(printfStr, validatedElem, strings.Repeat("-", len(e)-indexes[len(indexes)-1][1]))

		h = fmt.Sprintf(printfStr, h, validatedElem)
	}

	// full hostname must be <= 64 char long
	return trimString(h, 64), nil
}

func trimString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func configRoutes(routes []vcfg.Route) {

	for _, r := range routes {

		dst, nw, err := net.ParseCIDR(r.Destination)
		if err != nil {
			logError("can not set route destination: %v", r.Destination)
			continue
		}

		gw := net.ParseIP(r.Gateway)
		if gw == nil {
			logError("gateway %s invalid", r.Gateway)
			continue
		}

		// check if gateway is in that network
		// if not, we need to create a direct link
		if !nw.Contains(gw) {
			err := addNetworkRoute4(gw, net.IPv4bcast, nil, r.Interface,
				unix.RTF_UP|unix.RTF_HOST)
			if err != nil {
				logError("can not set route direct link: %v", err)
				continue
			}
		} else {
			err := addNetworkRoute4(gw, net.IP(nw.Mask), nil, r.Interface,
				unix.RTF_UP|unix.RTF_HOST)
			if err != nil {
				logError("can not set route in network: %v", err)
				continue
			}
		}

		err = addNetworkRoute4(dst, net.IP(nw.Mask), gw, r.Interface,
			unix.RTF_UP|unix.RTF_STATIC|unix.RTF_GATEWAY)
		if err != nil {
			logError("can not set route: %v", err)
			continue
		}

	}

}
