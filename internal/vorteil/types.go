package vorteil

import (
	"net"

	"github.com/vorteil/vorteil/pkg/vcfg"
)

type LogLevel int
type hypervisor int
type cloud int
type bFieldType int
type status int
type logType int

const (
	STATUS_SETUP    status = iota
	STATUS_RUN      status = iota
	STATUS_POWEROFF status = iota
)

const (
	BOOTSTRAP_NULL                  bFieldType = iota
	BOOTSTRAP_SLEEP                            = iota
	BOOTSTRAP_FIND_AND_REPLACE                 = iota
	BOOTSTRAP_DOWNLOAD                         = iota
	BOOTSTRAP_DEFINE_IF_NOT_DEFINED            = iota
	BOOTSTRAP_WAIT_PORT                        = iota
	BOOTSTRAP_WAIT_FILE                        = iota
)

// const (
// 	LOG_SYSTEM   logType = 1
// 	LOG_KERNEL   logType = 2
// 	LOG_STDOUT   logType = 3
// 	LOG_PROGRAMS logType = 4
// 	LOG_ALL      logType = 7
// )

const (
	ENV_HYPERVISOR     = "HYPERVISOR"
	ENV_CLOUD_PROVIDER = "CLOUD_PROVIDER"
	ENV_ETH_COUNT      = "ETH_COUNT"
	ENV_HOSTNAME       = "HOSTNAME"
	ENV_EXT_HOSTNAME   = "EXT_HOSTNAME"
	ENV_IP             = "IP%d"
	ENV_EXT_IP         = "EXT_IP%d"
	ENV_USERDATA       = "USERDATA"
)

const (
	LOG_EMERG   LogLevel = iota
	LOG_ALERT            = iota
	LOG_CRIT             = iota
	LOG_ERR              = iota
	LOG_WARNING          = iota
	LOG_NOTICE           = iota
	LOG_INFO             = iota
	LOG_DEBUG            = iota
	LOG_STDERR           = iota
)

const (
	HV_UNKNOWN    hypervisor = iota
	HV_KVM        hypervisor = iota
	HV_VMWARE     hypervisor = iota
	HV_HYPERV     hypervisor = iota
	HV_VIRTUALBOX hypervisor = iota
	HV_XEN        hypervisor = iota
)

var (
	hypervisorStrings = map[hypervisor]string{
		HV_UNKNOWN:    "UNKNOWN",
		HV_KVM:        "KVM",
		HV_VMWARE:     "VMWARE",
		HV_HYPERV:     "HYPERV",
		HV_VIRTUALBOX: "VIRTUALBOX",
		HV_XEN:        "XEN",
	}

	cloudStrings = map[cloud]string{
		CP_UNKNOWN: "UNKNOWN",
		CP_NONE:    "NONE",
		CP_GCP:     "GCP",
		CP_AZURE:   "AZURE",
		CP_EC2:     "EC2",
	}

	initStatus = STATUS_SETUP
)

const (
	CP_UNKNOWN cloud = iota
	CP_NONE    cloud = iota
	CP_GCP     cloud = iota
	CP_EC2     cloud = iota
	CP_AZURE   cloud = iota
)

type logFn func(level LogLevel, format string, values ...interface{})

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

type Vinitd struct {
	diskname string
	hostname string

	user string

	hypervisorInfo hv

	vcfg vcfg.VCFG

	programs []*program
	ifcs     map[string]*ifc
	dns      []net.IP
}

type logEntry struct {
	logType    logType
	logStrings []string
}

// config space structs
type bootstrapInstruction struct {
	btype bFieldType
	time  uint32
	args  []string
}

type programConf struct {
	count  uint16
	values []string
}

type program struct {
	path     string
	vcfgProg vcfg.Program

	env  programConf
	args programConf
	// old

	vinitd *Vinitd

	status status

	fpath          string
	cwd            string
	stdout, stderr string
	privilege      byte // 0 = root, 1 = superuser, 2 = user
	strace         byte

	// env        programConf
	// args       programConf
	logs       programConf
	bootstrapc uint16
	bootstraps []*bootstrapInstruction
}

// vcfg
type BootloaderCfg struct {
	Version         [16]byte
	Rsvd_a          [12]byte
	Preload_sectors uint32
	Kernel_args_len uint16
	Rsvd_b          [222]byte
	Kernel_args     [256]byte
}

type LayoutRegion struct {
	Lba     uint32
	Sectors uint32
}

type DiskLayout struct {
	Config                         LayoutRegion // 512
	Kernel                         LayoutRegion // 520
	_/*Trampoline*/ LayoutRegion   // 528
	_/*Variables*/ LayoutRegion    // 536
	_/*Arguments*/ LayoutRegion    // 544
	InitdConfig                    LayoutRegion // 552
	LoggingConfig                  LayoutRegion // 560
	_                              [8]byte      // 568
	VCFGTOML                       LayoutRegion // 576
	_/*Application*/ LayoutRegion  // 584
	_/*ScratchSpace*/ LayoutRegion // 592
	GoConfig                       LayoutRegion // 600
	Rsvd_a                         [24]byte     // 608
	Fs                             LayoutRegion // 632
	Rsvd_c                         [384]byte    // 640
}

type NTPSrv [256]byte

type KernelCfg struct {
	BootDelay uint32
	MaxFds    uint32
	LogFormat uint16
	LogType   uint16
	Rsvd_a    [756]byte
	NTP       [5]NTPSrv
}

type AppCfg struct {
	ElfMem      uint32
	Rsvd_a      [60]byte
	MetaVersion uint8
	Name        [64]byte
	Author      [128]byte
	Version     [64]byte
	Date        uint64
	URL         [256]byte
	Summary     [280]byte
	Kernel      [16]byte
	/* the application data below is not guaranteed to be reliable: they
	contain values from the package VCFG, not the final values used by the
	compiler or the hypervisor */
	Cpus         uint8
	Ram          uint32
	Inodes       uint32
	DiskSize     uint32
	NetworkPorts [96]byte
	Rsvd_b       [34]byte
}

type NFSMount struct {
	MountPoint [128]byte
	Srv        [128]byte
	Attrs      [256]byte
}

type VfsCfg struct {
	FsType [8]byte
	Rsvd_a [2040]byte
	NFS    [4]NFSMount
}

type IfaceCfg struct {
	IP         uint32
	Mask       uint32
	Gateway    uint32
	MTU        uint16
	TSOEnabled byte
	TCPDump    byte
	Rsvd_a     [240]byte
}

type IfaceRoute struct {
	Iface uint32
	Dst   uint32
	Gw    uint32
	Mask  uint32
}

type NetCfg struct {
	Hostname [256]byte
	DNS      [4]uint32
	Rsvd_a   [752]byte
	Iface    [4]IfaceCfg
	Route    [16]IfaceRoute
	Rsvd_b   [1792]byte
}

type GoCfg struct {
	User   string            `json:"user"`
	Sysctl map[string]string `json:"sysctl"`
}

type PersistedConf struct {
	Boot     BootloaderCfg
	Disk     DiskLayout
	Kernel   KernelCfg
	App      AppCfg
	Vfs      VfsCfg
	Net      NetCfg
	Rsvd_end [4096]byte
}

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
	Guid           [16]byte
	StartLBAParts  uint64
	NoOfParts      uint32
	SizePartEntry  uint32
	CrcParts       uint32
	Reserved1      [420]byte
}

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

type PartitionEntry struct {
	TypeGUID [16]byte
	PartGUID [16]byte
	FirstLBA uint64
	LastLBA  uint64
	Attributes/*attributes*/ uint64
	Name [72]byte
}
