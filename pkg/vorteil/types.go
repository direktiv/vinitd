/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"net"
	"os"
	"os/exec"

	"github.com/vorteil/vorteil/pkg/vcfg"
)

type hypervisor int
type cloud int
type bFieldType int
type status int
type logType int

const (
	statusSetup    status = iota
	statusRun      status = iota
	statusLaunched status = iota
	statusPoweroff status = iota
	statusError    status = iota
)

const (
	bootstrapSleep    = "SLEEP"
	bootstrapFandR    = "FIND_AND_REPLACE"
	bootstrapDefine   = "DEFINE_IF_NOT_DEFINED"
	bootstrapWaitFile = "WAIT_FILE"
	bootstrapWaitPort = "WAIT_PORT"
	bootstrapGet      = "GET"
)

const (
	envHypervisor    = "HYPERVISOR"
	envCloudProvider = "CLOUD_PROVIDER"
	envEthCount      = "ETH_COUNT"
	envHostname      = "HOSTNAME"
	envExtHostname   = "EXT_HOSTNAME"
	envIP            = "IP%d"
	envExtIP         = "EXT_IP%d"
	envUserData      = "USERDATA"
)

const (
	hvUnknown hypervisor = iota
	hvKVM     hypervisor = iota
	hvVMWare  hypervisor = iota
	hvHyperV  hypervisor = iota
	hvVBox    hypervisor = iota
	hvXen     hypervisor = iota
)

var (
	hypervisorStrings = map[hypervisor]string{
		hvUnknown: "UNKNOWN",
		hvKVM:     "KVM",
		hvVMWare:  "VMWARE",
		hvHyperV:  "HYPERV",
		hvVBox:    "VIRTUALBOX",
		hvXen:     "XEN",
	}

	cloudStrings = map[cloud]string{
		cpUnknown: "UNKNOWN",
		cpNone:    "NONE",
		cpGCP:     "GCP",
		cpAzure:   "AZURE",
		cpEC2:     "EC2",
	}

	initStatus = statusSetup
)

const (
	cpUnknown cloud = iota
	cpNone    cloud = iota
	cpGCP     cloud = iota
	cpEC2     cloud = iota
	cpAzure   cloud = iota
)

type ifc struct {
	name   string
	idx    int
	netIfc net.Interface
	addr   *net.IPNet
	gw     net.IP
}

type hv struct {
	hypervisor hypervisor
	cloud      cloud
	envs       map[string]string
}

// Vinitd contains all information to run and manage this instance
type Vinitd struct {
	diskname string
	hostname string

	// user running applications
	user string

	hypervisorInfo hv

	vcfg vcfg.VCFG

	// programs to run
	programs []*program

	// interfaces list
	ifcs map[string]*ifc

	// configured dns servers
	dns []net.IP

	tty, ttyS, ttyRedir *os.File

	readOnly        bool
	instantShutdown bool
}

type program struct {
	path     string
	vcfgProg vcfg.Program

	env  []string
	args []string
	logs []string

	// cmd.Process is not nil once started. app counter uses this
	cmd *exec.Cmd

	vinitd *Vinitd

	reaper bool
}

// GPTHeader for disk expansion
type GPTHeader struct {
	Signature      uint64
	Revision       [4]byte
	HeaderSize     uint32
	Crc            uint32
	Reserved0      uint32
	CurrentLBA     uint64
	BackupLBA      uint64
	FirstUsableLBA uint64
	LastUsableLBA  uint64
	GUID           [16]byte
	StartLBAParts  uint64
	NoOfParts      uint32
	SizePartEntry  uint32
	CrcParts       uint32
	Reserved1      [420]byte
}

// ProtectiveMBREntry for disk expansion
type ProtectiveMBREntry struct {
	Bootloader [446]byte
	Status     byte
	Reserved0/*headFirst*/ byte
	Reserved1/*sectorFirst*/ byte
	Reserved2/*cylinderFirst*/ byte
	PartitionType byte
	Reserved3/*headLast*/ byte
	Reserved4/*sectorLast*/ byte
	Reserved5/*cylinderLast*/ byte
	FirstLBA        uint32
	NumberOfSectors uint32
	Reserved6       [48]byte
	MagicNumber     [2]byte
}

// PartitionEntry for disk expansion
type PartitionEntry struct {
	TypeGUID [16]byte
	PartGUID [16]byte
	FirstLBA uint64
	LastLBA  uint64
	Attributes/*attributes*/ uint64
	Name [72]byte
}
