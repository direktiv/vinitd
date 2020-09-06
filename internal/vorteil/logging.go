package vorteil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/vorteil/vorteil/pkg/vcfg"
	"golang.org/x/sys/unix"
)

const (
	fluentbitApp = "/vorteil/fluent-bit"

	inputString = "[INPUT]\n"
	vlogDir     = "/vlogs"
	vlogType    = "vlogfs"

	logSystem = "system"
	logKernel = "kernel"
	logStdout = "stdout"
	logProgs  = "programs"
	logAll    = "all"
)

func addLogginOutput(sb *strings.Builder, logEntry vcfg.Logging, match string) {
	sb.WriteString("[OUTPUT]\n")

	for _, l := range logEntry.Config {
		s := strings.SplitN(l, "=", 2)
		if len(s) != 2 {
			logError("can not add logging output for %s", s)
		}
		sb.WriteString(fmt.Sprintf("    %s %s\n", s[0], s[1]))
	}

	sb.WriteString(fmt.Sprintf("    Match %s\n", match))

}

func addSystemLogging(sb *strings.Builder, ifcs map[string]*ifc) {

	addInput := func(sb *strings.Builder, name string) {
		sb.WriteString(inputString)
		sb.WriteString(fmt.Sprintf("    Name %s\n", name))
		sb.WriteString("    Tag vsystem\n")
	}

	addInput(sb, "cpu")
	addInput(sb, "disk")
	addInput(sb, "mem")
	addInput(sb, "vdisk")

	// interfaces
	for _, ifc := range ifcs {
		sb.WriteString(inputString)
		sb.WriteString("    Name netif\n")
		sb.WriteString(fmt.Sprintf("    Interface %s\n", ifc.name))
		sb.WriteString("    Tag vsystem\n")
	}

}

func addKernelLogging(sb *strings.Builder) {

	sb.WriteString(inputString)
	sb.WriteString("    Name kmsg\n")
	sb.WriteString("    Tag vkernel\n")

}

func addStdoutLogging(sb *strings.Builder) {

	os.Mkdir(vlogDir, 0755)
	mountFs(vlogDir, vlogType, "")

	file, err := os.OpenFile(defaultTTY, os.O_RDWR, 0)
	if err != nil {
		logError("can not open vtty: %s", err.Error())
		return
	}
	defer file.Close()

	mode := 4
	_, _, ep := unix.Syscall(unix.SYS_IOCTL, file.Fd(),
		MSG_IOCTL_OUTPUT, uintptr(unsafe.Pointer(&mode)))
	if ep != 0 {
		if err != nil {
			logError("can not ioctl vtty: %s", err.Error())
			return
		}
	}

	sb.WriteString(inputString)
	sb.WriteString("    Name tail\n")
	sb.WriteString("    Refresh_Interval 10\n")
	sb.WriteString(fmt.Sprintf("    Path %s/stdout\n", vlogDir))
	sb.WriteString("    Path_Key filename\n")
	sb.WriteString("    Skip_Long_Lines On\n")
	sb.WriteString("    Tag vstdout\n")

}

func addProgLogging(sb *strings.Builder, programs []*program) {

	os.Mkdir(vlogDir, 0755)
	mountFs(vlogDir, vlogType, "")

	for _, a := range programs {
		for _, l := range a.logs.values {

			dir := filepath.Dir(l)
			logDebug("creating logging dir %v", dir)
			os.Mkdir(dir, 0700)

			files, err := ioutil.ReadDir(dir)
			if err != nil {
				logError("can not read directory for logging: %s", err.Error())
				continue
			}

			if len(files) > 0 {
				logWarn("logging directory not empty, using real directory")
			} else {
				logDebug("mounting %s as log dir", dir)
				mountFs(dir, vlogType, "")
			}

			os.Chown(dir, userID, userID)

			sb.WriteString(inputString)
			sb.WriteString("    Name tail\n")
			sb.WriteString(fmt.Sprintf("    Path %s\n", l))
			sb.WriteString("    Path_Key filename\n")
			sb.WriteString("    Skip_Long_Lines On\n")
			sb.WriteString("    Tag vprog\n")

		}
	}
}

func (v *Vinitd) startLogging() {

	writeEtcFile("parsers.conf", filepath.Join("/etc", "parsers.conf"))

	var str strings.Builder
	str.WriteString("[SERVICE]\n")
	str.WriteString("    Flush 10\n")
	str.WriteString("    Daemon off\n")
	str.WriteString("    Log_Level warn\n")
	str.WriteString("    Parsers_File  /etc/parsers.conf\n")

	for _, l := range v.vcfg.Logging {

		switch l.Type {
		case logSystem:
			{
				addSystemLogging(&str, v.ifcs)
				addLogginOutput(&str, l, "vsystem")
			}
		case logKernel:
			{
				addKernelLogging(&str)
				addLogginOutput(&str, l, "vkernel")
			}
		case logStdout:
			{
				addStdoutLogging(&str)
				addLogginOutput(&str, l, "vstdout")
			}
		case logProgs:
			{
				// TODO programs
				// addProgLogging(&str, v.programs)
				addLogginOutput(&str, l, "vprog")
			}
		case logAll:
			{
				addSystemLogging(&str, v.ifcs)
				addKernelLogging(&str)
				addStdoutLogging(&str)
				// addProgLogging(&str, v.programs)
				addLogginOutput(&str, l, "*")
			}
		}

	}
	//
	// str.WriteString("[FILTER]\n")
	// str.WriteString("    Name record_modifier\n")
	// str.WriteString("    Match *\n")
	// str.WriteString("    Record hostname ${HOSTNAME}\n")
	//
	// err := ioutil.WriteFile("/etc/fb.cfg", []byte(str.String()), 0644)
	// if err != nil {
	// 	logAlways("can not create fluent-bit config file: %s", err.Error())
	// 	return
	// }
	//
	// cmd := exec.Command("/vorteil/fluent-bit", fmt.Sprintf("--config=/etc/fb.cfg"), fmt.Sprintf("--plugin=/vorteil/flb-in_vdisk.so"))
	//
	// stderr, err := os.OpenFile("/dev/null", os.O_WRONLY|os.O_APPEND, 0)
	// if err != nil {
	// 	logError("can not create fluentbit stderr: %s", err.Error())
	// 	return
	// }
	//
	// stdout, err := os.OpenFile("/dev/null", os.O_WRONLY|os.O_APPEND, 0)
	// if err != nil {
	// 	logError("can not cr	eate fluentbit stdout: %s", err.Error())
	// 	return
	// }
	//
	// cmd.Stderr = stderr
	// cmd.Stdout = stdout
	//
	// cmd.Env = []string{fmt.Sprintf("HOSTNAME=%s", v.hostname)}
	//
	// err = cmd.Start()
	// if err != nil {
	// 	logAlways("%s", err.Error())
	// }

}
