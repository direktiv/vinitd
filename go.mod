module github.com/vorteil/vinitd

go 1.14

replace github.com/insomniacslk/dhcp => ./build/dhcp

replace github.com/Asphaltt/dnsproxy-go => ./build/dnsproxy-go

replace github.com/vorteil/vorteil => /home/jensg/go/src/github.com/vorteil/vorteil

require (
	github.com/Asphaltt/dnsproxy-go v0.0.0-20181028064240-4c302a933bd0
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20200601194411-4b5a011e0a4c
	github.com/kr/pretty v0.1.0 // indirect
	github.com/miekg/dns v1.1.31 // indirect
	github.com/mitchellh/go-ps v1.0.0
	github.com/rakyll/statik v0.1.7
	github.com/stretchr/testify v1.6.1
	github.com/vishvananda/netlink v1.1.0
	github.com/vorteil/vorteil v0.0.0-00010101000000-000000000000
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/sys v0.0.0-20200817155316-9781c653f443
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
)
