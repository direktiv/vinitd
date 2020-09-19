module github.com/vorteil/vinitd

go 1.14

replace github.com/insomniacslk/dhcp => ./build/dhcp

replace github.com/Asphaltt/dnsproxy-go => ./build/dnsproxy-go

require (
	github.com/Asphaltt/dnsproxy-go v0.0.0-20181028064240-4c302a933bd0
	github.com/hashicorp/go-reap v0.0.0-20170704170343-bf58d8a43e7b
	github.com/insomniacslk/dhcp v0.0.0-20200601194411-4b5a011e0a4c
	github.com/miekg/dns v1.1.31 // indirect
	github.com/mitchellh/go-ps v1.0.0
	github.com/rakyll/statik v0.1.7
	github.com/stretchr/testify v1.6.1
	github.com/vishvananda/netlink v1.1.0
	github.com/vorteil/vorteil v0.0.0-20200918040815-3e9233b3cf35
	golang.org/x/sys v0.0.0-20200817155316-9781c653f443
)
