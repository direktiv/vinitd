VORTEIL_BIN := 'cli'
BUNDLER   := 'master'

basedir := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))

.PHONY: all
all: prep statik build

.PHONY: build
build: prep
	export CGO_LDFLAGS="-static -w -s" && \
	go build -tags osusergo,netgo -o build/vinitd cmd/vorteil.go

.PHONY: clean
clean:
	rm -rf build/*

.PHONY: statik
statik:
	@mkdir -p $(basedir)/build/
	@if [ ! -d "build/statik" ]; then \
		echo "creating statik file $(basedir)"; \
		cd $(basedir)/build && git clone https://github.com/rakyll/statik.git; \
		cd $(basedir)/build/statik && go build; \
		$(basedir)/build/statik/statik -f -include  *.dat -p vorteil -dest internal -src $(basedir)/assets/etc; \
	fi
.PHONY: prep
prep: dns dhcp statik

.PHONY: build-bundler
build-bundler:
	echo "checking bundler"
	@mkdir -p $(basedir)/build/
	@if [ ! -d "build/bundler" ]; then \
	    echo 'downloading bundler'; \
			cd $(basedir)/build/ && git clone --single-branch --branch=${BUNDLER} https://github.com/vorteil/bundler.git --depth 1; \
			cd $(basedir)/build/bundler && go build -o bundler cmd/main.go; \
	fi

.PHONY: bundle
bundle: build-bundler build
	@if [ ! -n "$$BUNDLE" ] || [ ! -n "$$VERSION" ]  || [ ! -n "$$TARGET" ]; then \
	    echo 'BUNDLE, VERSION or TARGET not set, e.g. make BUNDLE=20.9.2 VERSION=99.99.1 TARGET=/tmp bundle'; \
			exit 1; \
	fi
	@echo "using bundle $(BUNDLE)"
	@mkdir -p $(basedir)/build/bundle
	@mkdir -p $(basedir)/build/bundle/files
	@if [ ! -f /build/bundle/kernel-$(BUNDLE) ]; then \
		echo "downloading bundle $(BUNDLE) to build/bundle/kernel-$(BUNDLE)"; \
	fi
	@if [ ! -f "build/bundle/files/bundle.toml" ]; then \
		echo "extracting bundle"; \
		$(basedir)/build/bundler/bundler extract build/bundle/kernel-$(BUNDLE) build/bundle/files; \
	fi
	cp build/vinitd build/bundle/files
	$(basedir)/build/bundler/bundler create $(VERSION) build/bundle/files/bundle.toml > $(TARGET)/kernel-$(VERSION)

.PHONY: dns
dns:
	echo "checking dns"
	@if [ ! -d build/dnsproxy-go ]; 													\
		then																	\
			 mkdir -p build && cd build &&	\
			 git clone https://github.com/vorteil/dnsproxy-go; \
	fi

.PHONY: dhcp
dhcp:
	echo "checking dhcp"
	@if [ ! -d build/dhcp ]; 													\
		then																	\
			 mkdir -p build && cd build &&	\
			 git clone https://github.com/vorteil/dhcp.git; \
	fi

.PHONY: test
test:
	@echo "running tests"
	@if [ ! -d test/dl ]; 													\
		then	\
		echo "getting go alpine"; \
		$(VORTEIL_BIN) projects convert-container golang:alpine test/dl; \
	fi
	@rm -Rf test/base
	@$(VORTEIL_BIN) run -v --record=test/base  --files=. --program[0].binary="/test/run_prep.sh" --vm.ram="2048MiB" --vm.cpus=4 --vm.inodes=200000 --vm.disk-size="+1024MiB" --vm.kernel=99.99.1 test/dl
	# $(VORTEIL_BIN) run -v --files=. --program[0].binary="/test/run_tests.sh" --vm.ram="2048MiB" --vm.cpus=4 --vm.inodes=200000 --vm.disk-size="+1024MiB" --vm.kernel=99.99.1 test/base
