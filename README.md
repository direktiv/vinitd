<br />
<p align="center">
  <a href="https://github.com/vorteil/vinitd">
    <img src="assets/images/vlogo.png" alt="vinitd">
  </a>
  <h3 align="center">vinitd - vorteil.io init</h3>
  <h5 align="center">vorteil.io init</h5>
</p>
<hr/>

Vinitd is the init process for [vorteil.io micro virtual machines](https://github.com/vorteil/vorteil). It manages the configuration of the environment and the applications on the instance. For more documentations: [TODO DOCS]()

### Architecture

Vinitd is a small but feature-complete init for small virtual machines which can be found at _/vorteil/vinitd_ on vorteil images. It is a purpose-built and and requires a specific disk-layout build by [vorteil's tools](https://github.com/vorteil/vorteil).

#### Disk Layout

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

This project is getting build during the bundle process of [vbundler](https://github.com/vorteil/vbundler) and [vbundler](https://github.com/vorteil/vbundler) has dedicated targets for building and updating _vinitd_ ('make dev-vinitd' in [vbundler](https://github.com/vorteil/vbundler)). This is the preferred method.

Nevertheless this project can be build standalone with 'make' and 'go' installed:

```sh
git clone https://github.com/vorteil/vinitd
cd vinitd
make
```

### License

Distributed under the Apache 2.0 License. See `LICENSE` for more information.

### Acknowledgements

* [dnsproxy](https://github.com/Asphaltt/dnsproxy-go)
* [dhcp](https://github.com/insomniacslk/dhcp)
