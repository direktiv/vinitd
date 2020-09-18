# statik needs the go binary directory
ifndef GOBINARYDIR
	GOBINARYDIR := ~/go/bin
endif

BUNDLER   := 'master'

.PHONY: all
all: prep etc build

.PHONY: build
build:
	export CGO_LDFLAGS="-static -w -s" && \
	go build -tags netgo -o build/vinitd cmd/vorteil.go

.PHONY: clean
clean:
	rm -rf build/*

.PHONY: etc
etc:
	go get github.com/miekg/dns
	go get github.com/rakyll/statik
	$(GOBINARYDIR)/statik -f -include  *.dat -p vorteil -dest internal -src assets/etc

.PHONY: prep
	prep: dns dhcp

.PHONY: build-bundler
build-bundler:
	@if [ ! -d "build/bundler" ]; then \
	    echo 'downloading bundler'; \
			cd build/ && git clone --single-branch --branch=${BUNDLER} https://github.com/vorteil/bundler.git --depth 1; \
			cd bundler && go build -o bundler cmd/main.go; \
	fi

.PHONY: bundle
bundle: build-bundler build
	@if [ ! -n "$$BUNDLE" ] || [ ! -n "$$VERSION" ]  || [ ! -n "$$TARGET" ]; then \
	    echo 'BUNDLE, VERSION or TARGET not set, e.g. make BUNDLE=20.9.2 VERSION=99.99.1 TARGET=/tmp bundle'; \
			exit 1; \
	fi
	@echo "using bundle $(BUNDLE)"
	@mkdir -p build/bundle
	@mkdir -p build/bundle/files
	@if [ ! -f build/bundle/kernel-$(BUNDLE) ]; then \
		echo "downloading bundle $(BUNDLE) to build/bundle/kernel-$(BUNDLE)"; \
	fi
	@if [ ! -f "build/bundle/files/bundle.toml" ]; then \
		echo "extracting bundle"; \
		build/bundler/bundler extract build/bundle/kernel-$(BUNDLE) build/bundle/files; \
	fi
	cp build/vinitd build/bundle/files
	build/bundler/bundler create $(VERSION) build/bundle/files/bundle.toml > $(TARGET)/kernel-$(VERSION)

.PHONY: dns
dns:
	@if [ ! -d build/dnsproxy-go ]; 													\
		then																	\
			 mkdir -p build && cd build &&	\
			 git clone https://github.com/vorteil/dnsproxy-go; \
	fi

.PHONY: dhcp
dhcp:
	@if [ ! -d build/dhcp ]; 													\
		then																	\
			 mkdir -p build && cd build &&	\
			 git clone https://github.com/vorteil/dhcp.git; \
	fi
