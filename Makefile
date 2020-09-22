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
	rm -rf $(basedir)/build/*

.PHONY: statik
statik:
	@mkdir -p $(basedir)/build/
	@if [ ! -d "$(basedir)/build/statik" ]; then \
		echo "creating statik file $(basedir)"; \
		cd $(basedir)/build && git clone https://github.com/rakyll/statik.git; \
		cd $(basedir)/build/statik && go build; \
	fi
	@echo "generating statik files"
	$(basedir)/build/statik/statik -f -include  *.dat -p vorteil -dest $(basedir)/pkg -src $(basedir)/assets/etc

.PHONY: prep
prep: dns dhcp build-bundler statik

.PHONY: build-bundler
build-bundler:
	@echo "checking bundler"
	@mkdir -p $(basedir)/build/
	@if [ ! -d "build/bundler" ]; then \
	    echo 'downloading bundler'; \
			cd $(basedir)/build/ && git clone --single-branch --branch=${BUNDLER} https://github.com/vorteil/bundler.git --depth 1; \
			cd $(basedir)/build/bundler && go build -o bundler cmd/main.go; \
	fi

.PHONY: bundle
bundle: build-bundler
	@if [ ! -n "$$BUNDLE" ] || [ ! -n "$$VERSION" ]  || [ ! -n "$$TARGET" ]; then \
	    echo 'BUNDLE, VERSION or TARGET not set, e.g. make BUNDLE=20.9.2 VERSION=99.99.1 TARGET=/tmp bundle'; \
			exit 1; \
	fi
	@echo "using bundle $(BUNDLE)"
	@mkdir -p $(basedir)/build/bundle
	@mkdir -p $(basedir)/build/bundle/files
	@echo "checking $(basedir)/build/bundle/kernel-$(BUNDLE)"
	@if [ ! -f $(basedir)build/bundle/kernel-$(BUNDLE) ]; then \
		echo "downloading bundle $(BUNDLE) to build/bundle/kernel-$(BUNDLE)"; \
		wget -O $(basedir)/build/bundle/kernel-$(BUNDLE) https://github.com/vorteil/vbundler/releases/download/$(BUNDLE)/kernel-$(BUNDLE); \
	fi
	@if [ ! -f "$(basedir)/build/bundle/files/bundle.toml" ]; then \
		echo "extracting bundle"; \
		$(basedir)/build/bundler/bundler extract $(basedir)/build/bundle/kernel-$(BUNDLE) $(basedir)/build/bundle/files; \
	fi
	cp $(basedir)/build/vinitd $(basedir)/build/bundle/files
	$(basedir)/build/bundler/bundler create $(VERSION) $(basedir)/build/bundle/files/bundle.toml > $(TARGET)/kernel-$(VERSION)

.PHONY: dns
dns:
	@echo "checking dns in $(basedir)/build"
	@if [ ! -d $(basedir)/build/dnsproxy-go ]; 													\
		then																	\
			 mkdir -p $(basedir)/build && cd $(basedir)/build &&	\
			 git clone https://github.com/vorteil/dnsproxy-go; \
	fi

.PHONY: dhcp
dhcp:
	@echo "checking dhcp in $(basedir)/build"
	@if [ ! -d $(basedir)/build/dhcp ]; 													\
		then																	\
			 mkdir -p $(basedir)/build && cd $(basedir)/build &&	\
			 git clone https://github.com/vorteil/dhcp.git; \
	fi

.PHONY: test
test:
	@echo "running tests"
	@if [ ! -d $(basedir)/test/dl ]; 													\
		then	\
		echo "getting go alpine"; \
		$(VORTEIL_BIN) projects convert-container golang:alpine test/dl; \
	fi
	@if [ ! -d $(basedir)/test/base ]; 													\
		then	\
		echo "running prep"; \
# copy the build related files \
		cp Makefile test/dl; \
		cp test/run* test/dl; \
# copy the golang app for testing \
		mkdir -p test/dl/app; \
		cp -Rf pkg  test/dl/app; \
		cp -Rf cmd  test/dl/app; \
		cp go.* test/dl/app; \
# copy assets for statik to run \
		cp -Rf assets test/dl; \
		$(VORTEIL_BIN) run --record=test/base --program[0].binary="/run_prep.sh" --vm.ram="2048MiB" --vm.cpus=4 --vm.disk-size="+2048MiB" --vm.kernel=99.99.1 test/dl; \
	fi
# copy assets again for testing
	@cp -Rf pkg  test/base/app
	@cp -Rf cmd  test/base/app
	@cp go.* test/base/app
	@cp -Rf assets test/base/app
	@cp test/run* test/base
	@rm -f test/base/c.out
	@cp $(basedir)/test/dl/.vorteilproject test/base
# build disk
	$(VORTEIL_BIN) build -f -o test/disk.raw --format=raw --program[0].binary="/run_tests.sh" --vm.ram="2048MiB" --vm.cpus=4 --vm.disk-size="+1024MiB" --vm.kernel=99.99.1 test/base
# run tests with qemu
	qemu-system-x86_64 -cpu host -enable-kvm -no-reboot -machine q35 -smp 4 -m 2048 -serial stdio -display none -device virtio-scsi-pci,id=scsi -device scsi-hd,drive=hd0 -drive if=none,file=test/disk.raw,format=raw,id=hd0  -netdev user,id=network0 -device virtio-net-pci,netdev=network0,id=virtio0,mac=26:10:05:00:00:0a
	rm -f c.out
	cli images cp test/disk.raw /c.out .
