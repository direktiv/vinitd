<br />
<p align="center">
  <a href="https://github.com/vorteil/vinitd">
    <img src="assets/images/vlogo.png" alt="vinitd">
  </a>
  <h3 align="center">vinitd - vorteil.io init</h3>
  <h5 align="center">vorteil.io init</h5>
</p>
<hr/>

[![Build Status](https://travis-ci.org/vorteil/vinitd.svg?branch=master)](https://travis-ci.org/vorteil/vinitd) <a href="https://codeclimate.com/github/vorteil/vinitd/maintainability"><img src="https://api.codeclimate.com/v1/badges/4f33b7b5dc76ba2d26ae/maintainability" /></a> [![Go Report Card](https://goreportcard.com/badge/github.com/vorteil/vinitd)](https://goreportcard.com/report/github.com/vorteil/vinitd) [![](https://godoc.org/github.com/vorteil/vinitd/pkg/vorteil?status.svg)](http://godoc.org/github.com/vorteil/vinitd/pkg/vorteil) [![Discord](https://img.shields.io/badge/chat-on%20discord-6A7EC2)](https://discord.gg/VjF6wn4)

Vinitd is the init process for [vorteil.io micro virtual machines](https://github.com/vorteil/vorteil). It manages the configuration of the environment and the applications on the instance. For more information:

* The [Vorteil.io tools](https://github.com/vorteil/vorteil) project.
* The Vorteil [documentation](https://docs.vorteil.io/).
* The free Vorteil [apps repository](http://apps.vorteil.io/).
* The Vorteil [blog](https://blog.vorteil.io/).
* The [Godoc](https://godoc.org/github.com/vorteil/vorteil) library documentation.

### Architecture

Vinitd is a small but feature-complete init for small virtual machines which can be found at _/vorteil/vinitd_ on vorteil images. It is a purpose-built and and requires a specific disk-layout build by [vorteil's tools](https://github.com/vorteil/vorteil).

#### Stages

Vinitd runs through four stages starting with low level tasks like setting up stdout, network and logging. In the last stage the configured applications are getting launched. Should an error occur during one of the stages vinitd stops the virtual machine.

<p align="center">
    <img src="assets/images/vinitd_phases.png" alt="vinitd phases">
</p>

##### Pre-Setup

* Basic vtty setup
* Creates _/tmp_ if it does not exist
* Mount _/proc, /sys, /dev/pts_
* Init _/proc/self/fd_

##### Setup

* Read VCFG configuration from disk
* Setup vtty based on VCFG
* Setup signals for poweroff, reboot
* Configure power button
* DHCP / static IP
* Configure routes
* Setup shared memory
* Run sysctls and defaults
* Setup fake users
* Create /etc files if required

##### Post Setup

* Start DNS cache
* Mount NFS
* Enable fluentbit logging
* Add cloud environment variables (EXT_IP etc.)
* Start chronyd if NTP provided

##### Launch

* Launch applications
* Launch strace if configured
* Start application listener

### Building

To build and test changes in vinitd it needs to be part of a bundle. To make this process easier there is a dedicated make target available to build a bundle with the newly build vinitd.

```sh
make BUNDLE=20.9.8 VERSION=88.88.1 TARGET=~/.vorteild/kernels/watch bundle
vorteil run /path/to/my/app --vm.kernel=88.88.1
```

The variables to provide are:

* BUNDLE: Base used for the new bundle. Can be any bundle from vbundler [releases page](https://github.com/vorteil/vbundler/releases).
* VERSION: Version of the new bundle. Needs to have the following format XX.XX.X
* TARGET: The target directory for the new bundle. After a successful build there will be a file _`kernel-$VERSION`_ in that directory. The easiest way is to choose vorteil's watch directory so the version is immediatley available for vorteil tools.

### Running tests

Tests in vinitd covering network and operating system settings. Therefore they need to run in a virtual machine. There are two make targets available to run tests within a virtual machine (qemu installation required).

**make test**: Runs tests in two steps. The first steps converts an ubuntu docker image and installs all required dependencies for running and testing vinitd. This runs only the first time. To re-run this step delete the test/dl directory. In a second step the VM starts and runs the tests only. The test results are stored in _c.out_ in the root directory of the project.

**make fulltest**: Runs the above steps in one step. This is for travis CI where the two step process is not necessary.

### Code of Conduct

We have adopted the [Contributor Covenant](https://github.com/vorteil/.github/blob/master/CODE_OF_CONDUCT.md) code of conduct.

### Contributing

Any feedback and contributions are welcome. Read our [contributing guidelines](https://github.com/vorteil/.github/blob/master/CONTRIBUTING.md) for details.

### License

Distributed under the Apache 2.0 License. See `LICENSE` for more information.

### Acknowledgements

* [dnsproxy](https://github.com/Asphaltt/dnsproxy-go)
* [dhcp](https://github.com/insomniacslk/dhcp)
