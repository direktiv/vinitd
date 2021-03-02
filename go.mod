module github.com/vorteil/vinitd

go 1.14

replace github.com/insomniacslk/dhcp => ./build/dhcp

require (
	github.com/AdguardTeam/dnsproxy v0.35.1
	github.com/AdguardTeam/golibs v0.4.4
	github.com/Asphaltt/dnsproxy-go v0.0.0-20181028064240-4c302a933bd0
	github.com/capnm/sysinfo v0.0.0-20130621111458-5909a53897f3
	github.com/coredns/caddy v1.1.0
	github.com/coredns/coredns v1.8.1
	github.com/davecgh/go-spew v1.1.1
	github.com/google/uuid v1.1.2
	github.com/hashicorp/go-reap v0.0.0-20170704170343-bf58d8a43e7b
	github.com/insomniacslk/dhcp v0.0.0-20200601194411-4b5a011e0a4c
	github.com/mattn/go-shellwords v1.0.11
	github.com/mitchellh/go-ps v1.0.0
	github.com/rakyll/statik v0.1.7
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	github.com/t-tomalak/logrus-easy-formatter v0.0.0-20190827215021-c074f06c5816
	github.com/vishvananda/netlink v1.1.0
	github.com/vorteil/vorteil v0.0.0-20210104040243-9ef31da53e7e
	golang.org/x/sys v0.0.0-20201214210602-f9fddec55a1e
)
