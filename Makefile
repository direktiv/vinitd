VORTEIL_BIN := 'vorteil'
BUNDLER   := 'master'
BASEDIR := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))

.PHONY: all
all: prep statik build

.PHONY: build
build: prep
	export CGO_LDFLAGS="-static -w -s" && \
	go build -tags osusergo,netgo -o build/vinitd cmd/vorteil.go

.PHONY: clean
clean:
	rm -rf $(BASEDIR)/build/*
	rm -rf $(BASEDIR)/test/base
	rm -rf $(BASEDIR)/test/dl

.PHONY: statik
statik:
	@mkdir -p build/
	@if [ ! -d "build/statik" ]; then \
		echo "creating statik binary"; \
		cd build && git clone https://github.com/rakyll/statik.git; \
		cd statik && go build; \
	fi
	@echo "generating statik files"
	build/statik/statik -f -include  *.dat -p vorteil -dest $(BASEDIR)/pkg -src assets/etc

.PHONY: prep
prep: dns dhcp build-bundler statik

.PHONY: build-bundler
build-bundler:
	@echo "checking bundler"
	@mkdir -p $(BASEDIR)/build/
	@if [ ! -d "build/bundler" ]; then \
	    echo 'downloading bundler'; \
			cd $(BASEDIR)/build/ && git clone --single-branch --branch=${BUNDLER} https://github.com/vorteil/bundler.git --depth 1; \
			cd $(BASEDIR)/build/bundler && go build -o bundler cmd/main.go; \
	fi

.PHONY: bundle
bundle: build-bundler build
	@if [ ! -n "$$BUNDLE" ] || [ ! -n "$$VERSION" ]  || [ ! -n "$$TARGET" ]; then \
	    echo 'BUNDLE, VERSION or TARGET not set, e.g. make BUNDLE=20.9.2 VERSION=20.9.5 TARGET=/tmp bundle'; \
			exit 1; \
	fi
	@echo "using bundle $(BUNDLE)"
	@mkdir -p $(BASEDIR)/build/bundle
	@mkdir -p $(BASEDIR)/build/bundle/files
	@echo "checking $(BASEDIR)/build/bundle/kernel-$(BUNDLE)"
	@if [ ! -f $(BASEDIR)build/bundle/kernel-$(BUNDLE) ]; then \
		echo "downloading bundle $(BUNDLE) to build/bundle/kernel-$(BUNDLE)"; \
		wget -O $(BASEDIR)/build/bundle/kernel-$(BUNDLE) https://github.com/vorteil/vbundler/releases/download/$(BUNDLE)/kernel-$(BUNDLE); \
	fi
	rm -Rf $(BASEDIR)/build/bundle/files
	echo "extracting bundle"; \
	$(BASEDIR)/build/bundler/bundler extract $(BASEDIR)/build/bundle/kernel-$(BUNDLE) $(BASEDIR)/build/bundle/files; \
	cp $(BASEDIR)/build/vinitd $(BASEDIR)/build/bundle/files
	$(BASEDIR)/build/bundler/bundler create $(VERSION) $(BASEDIR)/build/bundle/files/bundle.toml > $(TARGET)/kernel-$(VERSION)

.PHONY: dns
dns:
	@echo "checking dns in $(BASEDIR)/build"
	@if [ ! -d $(BASEDIR)/build/dnsproxy-go ]; 													\
		then																	\
			 mkdir -p $(BASEDIR)/build && cd $(BASEDIR)/build &&	\
			 git clone https://github.com/vorteil/dnsproxy-go; \
	fi

.PHONY: dhcp
dhcp:
	@echo "checking dhcp in $(BASEDIR)/build"
	@if [ ! -d $(BASEDIR)/build/dhcp ]; 													\
		then																	\
			 mkdir -p $(BASEDIR)/build && cd $(BASEDIR)/build &&	\
			 git clone https://github.com/vorteil/dhcp.git; \
	fi

.PHONY: convert
convert:
	@if [ ! -d $(BASEDIR)/test/dl ]; 													\
		then	\
		echo "getting go alpine with $(VORTEIL_BIN)"; \
		$(VORTEIL_BIN) projects convert-container golang:alpine test/dl -j; \
	fi

.PHONY: fulltest
fulltest: convert
	@cp Makefile test/run* test/dl; \
	mkdir -p test/dl/app; \
	cp -Rf pkg cmd go.* test/dl/app; \
	cp -Rf assets Makefile test/run* test/dl; \
	rm -Rf test/full; \
	$(SUDO) $(VORTEIL_BIN) run -j  --record=test/full --program[0].binary="/run_full.sh" --vm.ram="3072MiB" --vm.cpus=1 --vm.disk-size="+3072MiB" --vm.kernel=20.9.7 test/dl; \
	cp test/full/c.out .

.PHONY: test
test: convert
	@if [ ! -d $(BASEDIR)/test/base ]; 													\
		then	\
		echo "running prep"; \
		mkdir -p test/dl/app; \
		cp -Rf pkg cmd go.* test/dl/app; \
		cp -Rf assets Makefile test/run* test/dl; \
		$(SUDO) $(VORTEIL_BIN) run -j --record=test/base --program[0].binary="/run_prep.sh" --vm.ram="3072MiB" --vm.cpus=1 --vm.disk-size="+2048MiB" --vm.kernel=20.9.7 test/dl; \
		cp $(BASEDIR)/test/dl/.vorteilproject test/base; \
	fi

	@cp -Rf pkg cmd assets go.* test/base/app; \
	cp test/run* test/base; \
	rm -Rf test/done; \
	rm -f test/base/c.out; \
	$(SUDO) $(VORTEIL_BIN) build -f -j -o disk.raw --format=raw --vm.disk-size="+1024MiB" --program[0].binary="/run_tests.sh" --vm.kernel=20.9.7 test/base; \
	$(SUDO) qemu-system-x86_64 -cpu host -enable-kvm -no-reboot -machine q35 -smp 1 -m 3072 -serial stdio -display none -device virtio-scsi-pci,id=scsi -device scsi-hd,drive=hd0 -drive if=none,file=./disk.raw,format=raw,id=hd0 -netdev user,id=network0 -device virtio-net-pci,netdev=network0,id=virtio0; \
	vorteil images cp ./disk.raw /c.out .
