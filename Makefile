# statik needs the go binary directory
ifndef GOBINARYDIR
	GOBINARYDIR := ~/go/bin
endif

.PHONY: all
all: prep etc
	export CGO_LDFLAGS="-static -w -s -Wl,--dynamic-linker=/vorteil/ld-linux-x86-64.so.2 -Wl,-rpath,/vorteil" && \
	go build -tags netgo -o build/vinitd cmd/vorteil.go

.PHONY: clean
clean:
	rm -rf build/*

.PHONY: etc
etc:
	go get github.com/miekg/dns
	go get github.com/rakyll/statik
	$(GOBINARYDIR)/statik -f -include  *.dat -p vorteil -dest internal -src assets

.PHONY: prep
prep: dns dhcp

.PHONY: dns
dns:
	@if [ ! -d build/dnsproxy-go ]; 													\
		then																	\
			 cd build &&	\
			 git clone https://github.com/vorteil/dnsproxy-go; \
	fi

.PHONY: dhcp
dhcp:
	@if [ ! -d build/dhcp ]; 													\
		then																	\
			 cd build &&	\
			 git clone https://github.com/vorteil/dhcp.git; \
	fi
