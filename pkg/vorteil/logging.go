/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vorteil/vorteil/pkg/vcfg"
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

var (
	logInputs map[string]string
)

func addLogginOutput(sb *strings.Builder, logEntry vcfg.Logging,
	match string, envs map[string]string) {
	sb.WriteString("[OUTPUT]\n")

	for _, l := range logEntry.Config {
		s := strings.SplitN(l, "=", 2)
		if len(s) != 2 {
			logError("can not add logging output for %s", s)
		}

		// replace variables
		if strings.HasPrefix(s[1], "$") {
			env := s[1][1:]
			for k, e := range envs {
				if k == env {
					logDebug("replacing $%s for %s", env, e)
					s[1] = e
				}
			}
		}

		sb.WriteString(fmt.Sprintf("    %s %s\n", s[0], s[1]))
	}

	sb.WriteString(fmt.Sprintf("    Match_Regex %s\n", match))

}

func addSystemLogging(sb *strings.Builder, ifcs map[string]*ifc) {

	addInput := func(sb *strings.Builder, name string) {

		// we dont' add it again
		if _, ok := logInputs[name]; ok {
			return
		}
		logInputs[name] = name

		sb.WriteString(inputString)
		sb.WriteString(fmt.Sprintf("    Name %s\n", name))
		sb.WriteString(fmt.Sprintf("    Tag vsystem-%s\n", name))
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
		sb.WriteString(fmt.Sprintf("    Tag vsystem-%s\n", ifc.name))
	}

}

func addKernelLogging(sb *strings.Builder) {

	// we dont' add it again
	if _, ok := logInputs["kmsg"]; ok {
		return
	}
	logInputs["kmsg"] = "kmsg"

	sb.WriteString(inputString)
	sb.WriteString("    Name kmsg\n")
	sb.WriteString("    Tag vkernel\n")

}

func addStdoutLogging(sb *strings.Builder) {

	// we dont' add it again
	if _, ok := logInputs["tail"]; ok {
		return
	}
	logInputs["tail"] = "tail"

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

		for _, l := range a.vcfgProg.LogFiles {

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

	// this is to detect duplicate inputs
	logInputs = make(map[string]string)

	writeEtcFile("parsers.conf", filepath.Join("/etc", "parsers.conf"))

	var str strings.Builder
	str.WriteString("[SERVICE]\n")
	str.WriteString("    Flush 10\n")
	str.WriteString("    Daemon off\n")
	str.WriteString("    Log_Level error\n")
	str.WriteString("    Parsers_File  /etc/parsers.conf\n")

	redir := func() {

		os.Mkdir(vlogDir, 0755)
		mountFs(vlogDir, vlogType, "")
		os.OpenFile("/vlogs/stdout", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)

		f, err := enableTTYRedir()
		if err != nil {
			logError("can not enable ttyRedir: %v", err)
		}
		v.ttyRedir = f
	}

	for _, l := range v.vcfg.Logging {

		logDebug("logging type: %s", l.Type)

		switch t := l.Type; {
		case t == logSystem:
			{
				addSystemLogging(&str, v.ifcs)
				addLogginOutput(&str, l, "vsystem-*", v.hypervisorInfo.envs)
			}
		case t == logKernel:
			{
				addKernelLogging(&str)
				addLogginOutput(&str, l, "vkernel", v.hypervisorInfo.envs)
			}
		case t == logStdout:
			{
				addStdoutLogging(&str)
				addLogginOutput(&str, l, "vstdout", v.hypervisorInfo.envs)
				redir()
			}
		case t == logProgs:
			{
				addProgLogging(&str, v.programs)
				addLogginOutput(&str, l, "vprog", v.hypervisorInfo.envs)
			}
		default:
			{
				addSystemLogging(&str, v.ifcs)
				addKernelLogging(&str)
				addStdoutLogging(&str)
				addProgLogging(&str, v.programs)
				addLogginOutput(&str, l, t, v.hypervisorInfo.envs)
				redir()
			}
		}

	}

	str.WriteString("[FILTER]\n")
	str.WriteString("    Name record_modifier\n")
	str.WriteString("    Match *\n")
	str.WriteString("    Record hostname ${HOSTNAME}\n")

	if v.hypervisorInfo.cloud == cpEC2 {
		if iid, ok := v.hypervisorInfo.envs[envInstanceID]; ok {
			str.WriteString("[FILTER]\n")
			str.WriteString("    Name record_modifier\n")
			str.WriteString("    Match *\n")
			str.WriteString(fmt.Sprintf("    Record  ec2_instance_id %s\n", iid))
		}
	}

	err := ioutil.WriteFile("/etc/fb.cfg", []byte(str.String()), 0644)
	if err != nil {
		logError("can not create fluent-bit config file: %s", err.Error())
		return
	}

	logDebug("logging conf: %s", str.String())

	cmd := exec.Command("/vorteil/fluent-bit", fmt.Sprintf("--config=/etc/fb.cfg"), "--quiet", fmt.Sprintf("--plugin=/vorteil/flb-in_vdisk.so"))

	stderr, err := os.OpenFile("/dev/vtty", os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		logError("can not create fluentbit stderr: %s", err.Error())
		return
	}

	stdout, err := os.OpenFile("/dev/vtty", os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		logError("can not create fluentbit stdout: %s", err.Error())
		return
	}

	cmd.Stderr = stderr
	cmd.Stdout = stdout

	cmd.Env = []string{fmt.Sprintf("HOSTNAME=%s", v.hostname), "HOME=/", "LD_LIBRARY_PATH=/vorteil"}

	err = cmd.Start()
	if err != nil {
		logError("%s", err.Error())
	}

}
