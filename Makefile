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
	rm -rf $(BASEDIR)/test/_*

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
prep: dhcp build-bundler statik

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
	    echo 'BUNDLE, VERSION or TARGET not set, e.g. make bundle BUNDLE=20.9.7 VERSION=99.99.1 TARGET=/tmp bundle'; \
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
	@if [ ! -d $(BASEDIR)/test/_dl ]; 													\
		then	\
		echo "getting go alpine with $(VORTEIL_BIN)"; \
		$(VORTEIL_BIN) projects convert-container golang:alpine test/_dl -j; \
	fi

.PHONY: fulltest
fulltest: convert
	@cp Makefile test/run* test/_dl; \
	mkdir -p test/_dl/app; \
	cp -Rf pkg cmd go.* test/_dl/app; \
	cp -Rf assets Makefile test/run* test/_dl; \
	rm -Rf test/full; \
	$(SUDO) $(VORTEIL_BIN) run -j --record=test/_full --program[0].binary="/run_full.sh" --vm.ram="3072MiB" --vm.cpus=1 --vm.disk-size="+3072MiB" --vm.kernel=20.9.9 test/_dl; \
	cp test/_full/c.out .

.PHONY: test
test: convert
	@if [ ! -d $(BASEDIR)/test/_base ]; 													\
		then	\
		echo "running prep"; \
		mkdir -p test/_dl/app; \
		cp -Rf pkg cmd go.* test/_dl/app; \
		cp -Rf assets Makefile test/run* test/_dl; \
		$(SUDO) $(VORTEIL_BIN) run -j --record=test/_base --program[0].binary="/run_prep.sh" --vm.ram="3072MiB" --vm.cpus=1 --vm.disk-size="+2048MiB" --vm.kernel=20.9.9 test/_dl; \
		cp $(BASEDIR)/test/_dl/.vorteilproject test/_base; \
	fi
	@if [ ! -d $(BASEDIR)/test/_hw ]; 													\
		then	\
		$(SUDO) $(VORTEIL_BIN) projects convert-container hello-world test/_hw; \
		$(SUDO) $(VORTEIL_BIN) build -f -o test/_base/hw.raw --format=raw test/_hw; \
	fi

	echo "copying files"
	@cp -Rf pkg cmd assets go.* test/_base/app; \
	cp test/run* test/_base; \
	rm -f test/_base/c.out; \

	$(SUDO) $(VORTEIL_BIN) build -f -o disk.raw --format=raw --vm.ram="3072MiB" --vm.disk-size="+512MiB" --program[0].args="/run_tests.sh" --vm.kernel=20.9.8 test/_base; \
	$(SUDO) qemu-system-x86_64 -cpu host -enable-kvm -no-reboot -machine q35 -smp 1 -m 3072 -serial stdio -display none -device virtio-scsi-pci,id=scsi -device scsi-hd,drive=hd0 -drive if=none,file=./disk.raw,format=raw,id=hd0 -netdev user,id=network0 -device virtio-net-pci,netdev=network0,id=virtio0; \
	vorteil images cp ./disk.raw /c.out .
