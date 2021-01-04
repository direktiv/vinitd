/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	reap "github.com/hashicorp/go-reap"
	"github.com/vorteil/vorteil/pkg/vcfg"
	"golang.org/x/sys/unix"
)

const (
	pathEnvName   = "PATH"
	pathSeperator = ":"
	replaceString = "$%s"
	environString = "%s=%s"

	rootID = 0
	userID = 1000
)

func pickFromEnv(env string, p vcfg.Program) string {
	for _, e := range p.Env {
		es := strings.SplitN(e, "=", 2)
		if es[0] == env {
			return es[1]
		}
	}
	return ""
}

// func calculatePath(p vcfg.Program) string {
func calculatePath(ppath string, p vcfg.Program) string {

	// nothing to calculate if absolute
	if path.IsAbs(ppath) {
		return ppath
	}

	// if that file exists we return
	if _, err := os.Stat(filepath.Join(p.Cwd, ppath)); err == nil {
		// if that file exists we return the absolute path
		// if there is an error we are trying to just add a leading /
		path, err := filepath.Abs(filepath.Join(p.Cwd, ppath))
		if err != nil {
			logError("can not create path for %s, err %v", ppath, err)
			return fmt.Sprintf("/%s", ppath)
		}
		return path
	}

	// last chance: maybe it is in PATH
	pathEnv := strings.Split(pickFromEnv(pathEnvName, p), pathSeperator)
	if len(pathEnv) > 0 {
		for _, c := range pathEnv {
			if _, err := os.Stat(filepath.Join(c, ppath)); err == nil {
				return filepath.Join(c, ppath)
			}
		}
	}

	return ""
}

func fixDefaults(p *vcfg.Program) {

	if p.Stderr == "" {
		p.Stderr = defaultTTY
	}

	if p.Stdout == "" {
		p.Stdout = defaultTTY
	}

	// Treat empty cwd as "/" because calculatePath Filepath.Join functions break
	// when joining empty cwd with relative path
	if p.Cwd == "" {
		p.Cwd = defaultCWD
	}

}

func (p *program) waitForApp(cmd *exec.Cmd) {

	logDebug("waiting for process %d", cmd.Process.Pid)
	err := cmd.Wait()
	if err != nil {
		logDebug("error while waiting: %s", err.Error())
	}

	// Returns exit status
	logDebug("process %d finished with %s", cmd.Process.Pid, cmd.ProcessState.String())

	p.isDone = true

	// just in case call it again
	handleExit(p.vinitd.programs)
}

func (p *program) launch(systemUser string) error {

	fixDefaults(&p.vcfgProg)

	// strace override
	if p.vcfgProg.Strace {
		p.args = append(p.args, "")
		copy(p.args[1:], p.args)
		p.args[0] = p.path
		p.path = "/vorteil/strace"
	}

	cmd := exec.Command(p.path, p.args...)
	cmd.Env = p.env
	cmd.Dir = p.vcfgProg.Cwd

	var (
		user string
		rid  int
	)

	switch p.vcfgProg.Privilege {
	case vcfg.SuperuserPrivilege: // superuser privilege
		user = fmt.Sprintf("%s (superuser)", systemUser)
		rid = userID
	case vcfg.UserPrivilege:
		user = systemUser
		rid = userID
	default: // root privilege
		rid = rootID
		user = "root"
	}

	// either root or uid 1000
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(rid), Gid: uint32(rid)},
	}

	if p.vcfgProg.Privilege == "superuser" {
		cmd.SysProcAttr.AmbientCaps = []uintptr{
			unix.CAP_CHOWN,
			unix.CAP_DAC_OVERRIDE,
			unix.CAP_DAC_READ_SEARCH,
			unix.CAP_FOWNER,
			unix.CAP_IPC_OWNER,
			unix.CAP_NET_ADMIN,
			unix.CAP_MKNOD,
			unix.CAP_NET_BIND_SERVICE,
			unix.CAP_NET_RAW,
			unix.CAP_SYS_ADMIN,
		}
	}

	logDebug("starting as %s, uid %d", user, rid)

	// Create stderr dir if it does not exists
	if _, err := os.Stat(filepath.Dir(p.vcfgProg.Stderr)); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(p.vcfgProg.Stderr), 0)
	}

	stderr, err := os.OpenFile(p.vcfgProg.Stderr, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}

	// Create stdout dir if it does not exists
	if _, err := os.Stat(filepath.Dir(p.vcfgProg.Stdout)); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(p.vcfgProg.Stdout), 0)
	}

	stdout, err := os.OpenFile(p.vcfgProg.Stdout, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0)
	if err != nil {
		return err
	}

	cmd.Stderr = stderr
	cmd.Stdout = stdout

	p.cmd = cmd

	err = cmd.Start()
	if err != nil {
		return err
	}

	go p.waitForApp(cmd)

	logDebug("started %s as pid %d", p.path, cmd.Process.Pid)

	return nil
}

// bootstrapWaitForFile hangs process until the file appears
// warns every 30 seconds.
func bootstrapWaitForFile(args []string, p *program) {

	args = args[1:]
	// check length of args to see atleast one path
	if len(args) != 1 {
		logError("bootstrap 'WAIT_FILE' needs one value")
		return
	}

	// loop through and stat the path to see when it exists
	count := 0
	for {
		if _, err := os.Stat(args[0]); err == nil {
			break
		}
		// check count log a warning saying file hasn't appeared in 30 seconds repeat action
		if count%30 == 0 && count > 0 {
			logWarn("bootstrap 'WAIT_FILE' file %s has not appeared yet", args[0])
		}
		count++
		time.Sleep(time.Second * 1)
	}

}

// bootstrapGet executes a get request and saves the file
func bootstrapGetRequest(args []string, p *program) {

	if len(args) < 3 {
		logError("bootstrap 'WAIT_GET' needs at least one url and one target file")
		return
	}

	logDebug("get request: %s", args[1])

	_, err := url.ParseRequestURI(args[1])
	if err != nil {
		logAlways("can not parse url: %s", err)
		return
	}

	resp, err := http.Get(args[1])
	if err != nil {
		logAlways("can not get url: %s", err)
		return
	}

	defer resp.Body.Close()

	err = os.MkdirAll(filepath.Dir(args[2]), 0755)
	if err != nil {
		logAlways("can not create dir %s: %s", filepath.Dir(args[2]), err)
		return
	}

	out, err := os.Create(args[2])
	if err != nil {
		logAlways("can not create file %s: %s", args[2], err)
		return
	}
	defer out.Close()
	io.Copy(out, resp.Body)

	logDebug("got %s to %s", args[1], args[2])
}

// bootstrapWaitForPort hands process until the ports appear for certain network types, timesout after 30 seconds
func bootstrapWaitForPort(args []string, p *program) {

	var (
		ifce *net.Interface
		err  error
		ief  string
	)

	args = args[1:]

	// b.args[0] should be ifce, b.args[1:] should be all network
	if len(args) < 2 {
		logError("bootstrap 'WAIT_PORT' needs at least one value to listen out for")
		return
	}

	idx := 0
	ief = "eth0"

	// interface to look on should always exist if they provide it if not its default eth0
	if strings.Contains(args[0], "if=") {
		ief = strings.Split(args[0], "=")[1]
		idx = 1
	}

	ifce, err = net.InterfaceByName(ief)

	if err != nil {
		logError("bootstrap 'WAIT_PORT' unable to fetch interface %s: %s", ief, err)
		return
	}

	// addresses for the said interface
	addrs, err := ifce.Addrs()
	if err != nil {
		logError("bootstrap 'WAIT_PORT' unable to read addresses for %s: %s", ifce, err)
		return
	}

	var (
		ip     string
		isIpv4 bool
	)
	// loop through addresses to find ipv4 address for interface
	for _, addr := range addrs {
		// get ip object of the address and check if ipv4
		ip, isIpv4 = checkIfIPV4(addr.String())
		if isIpv4 {
			break
		}
	}

	var wg sync.WaitGroup
	// Loop through ports and attempt connections
	for _, arg := range strings.Split(strings.TrimSpace(strings.Join(args[idx:], " ")), " ") {
		// listen for ports to be alive
		wg.Add(1)
		// go routine it to test all ports currently asked for
		// simple check for numbers
		_, err := strconv.Atoi(arg)
		if err != nil {
			logError("The value '%s' does not seem to be a port number", arg)
		}

		go listenForPort(fmt.Sprintf("%s:%s", ip, arg), &wg)
	}
	// wait till group resolves
	wg.Wait()
}

// listenForPort listens for port in a loop returns nothing but resolves wait group when function successfully dials port
func listenForPort(addrToCheck string, wg *sync.WaitGroup) {
	defer wg.Done()
	count := 0
	for {
		// attempt connection
		conn, err := net.Dial("tcp", addrToCheck)
		if err == nil {
			conn.Close()
			break
		}
		// log if port is not up every 30 seconds
		if count%30 == 0 {
			logWarn("bootstrap 'WAIT_PORT' tcp connection to '%s' has not come online", addrToCheck)
		}
		count++
		time.Sleep(time.Second * 1)
	}
}

// checkIfIPV4 returns converted addr to IP and true or false if it is ipv4
func checkIfIPV4(addr string) (string, bool) {
	ip, _, err := net.ParseCIDR(addr)
	if err != nil {
		logError("bootstrap 'WAIT_PORT' unable to ParseCIDR of address: %s", err)
	}
	return ip.String(), strings.Count(ip.String(), ":") < 2
}

func bootstrapNotdefined(args []string, p *program) string {

	args = args[1:]

	// if this is not a pair, we ignore
	if len(args) != 2 {
		logError("bootstrap 'DEFINE_IF_NOT_DEFINED' needs two values")
		return ""
	}

	// check if it has been set
	for _, val := range p.env {
		if strings.HasPrefix(val, fmt.Sprintf("%s=", args[0])) {
			return ""
		}
	}

	for k, val := range p.vinitd.hypervisorInfo.envs {
		logDebug("replacing %s with %s", fmt.Sprintf(replaceString, k), val)
		args[1] = strings.ReplaceAll(args[1], fmt.Sprintf(replaceString, k), val)
	}

	return fmt.Sprintf(environString, args[0], args[1])

}

// func bootstrapReplace(b *bootstrapInstruction) {
func bootstrapReplace(args []string, p *program) {

	args = args[1:]

	if len(args) != 3 {
		logError("bootstrap 'FIND_AND_REPLACE' needs two values")
		return
	}

	m := make(map[string]string)
	for _, a := range args {
		aa := strings.SplitN(a, "=", 2)
		m[aa[0]] = aa[1]
	}

	txt, err := ioutil.ReadFile(m["file"])
	if err != nil {
		logWarn("file %s does does not exist to replace text", m["file"])
		return
	}

	// check if it has been set
	for _, val := range p.env {
		s := strings.SplitN(val, "=", 2)
		m["replace"] = strings.ReplaceAll(m["replace"], fmt.Sprintf(replaceString, s[0]), s[1])
	}

	content := strings.Replace(string(txt), m["find"], m["replace"], -1)
	err = ioutil.WriteFile(m["file"], []byte(content), 0)
	if err != nil {
		return
	}

}

func (p *program) bootstrap() error {

	for _, b := range p.vcfgProg.Bootstrap {

		bs := strings.Split(b, " ")

		if len(bs) == 0 {
			logError("can not parse bootstrap %s", bs)
			continue
		}

		switch bs[0] {
		case bootstrapSleep:
			{
				s, err := strconv.Atoi(bs[1])
				if err != nil {
					logError("can not parse sleep bootstrap %s", bs)
					continue
				}
				time.Sleep(time.Duration(s) * time.Millisecond)
			}
		case bootstrapWaitFile:
			{
				bootstrapWaitForFile(bs, p)
				break
			}
		case bootstrapWaitPort:
			{
				bootstrapWaitForPort(bs, p)
				break
			}
		case bootstrapGet:
			{
				bootstrapGetRequest(bs, p)
				break
			}
		case bootstrapFandR:
			{
				bootstrapReplace(bs, p)
			}
		case bootstrapDefine:
			{
				s := bootstrapNotdefined(bs, p)
				logDebug("bootstrap not definded: %s", s)
				if len(s) > 0 {
					p.env = append(p.env, s)
				}

				// we need to repace it if required
				if len(p.args) > 1 {
					p.args = args(p.args[1:], p.env)
				}
			}
		default:
			{
				logError("unknown bootstrap command: %s", bs[0])
			}
		}
	}

	return nil
}

func args(progArgs []string, envs []string) []string {

	var newArgs []string

	// convert envs into map
	ee := map[string]string{}

	for _, s := range envs {
		// we can assume KEY=VALUE here
		k := strings.SplitN(s, "=", 2)
		ee[k[0]] = k[1]
	}

	for _, e := range progArgs {
		for k, val := range ee {
			e = strings.ReplaceAll(e, fmt.Sprintf(replaceString, k), val)
		}
		newArgs = append(newArgs, e)
	}

	return newArgs

}

func envs(progValues []string, hyperVisorEnvs map[string]string) []string {

	var newEnvs []string
	for _, e := range progValues {
		for k, val := range hyperVisorEnvs {
			e = strings.ReplaceAll(e, fmt.Sprintf(replaceString, k), val)
		}
		newEnvs = append(newEnvs, e)
	}

	// now add them as well
	for k, val := range hyperVisorEnvs {
		newEnvs = append(newEnvs, fmt.Sprintf(environString, k, val))
	}

	return newEnvs
}

func (v *Vinitd) prepProgram(p vcfg.Program) error {

	// we can add the program to the list now
	np := &program{
		vcfgProg: p,
		cmd:      nil,
		vinitd:   v,
	}

	v.programs = append(v.programs, np)

	return nil
}

func (v *Vinitd) launchProgram(np *program) error {

	p := np.vcfgProg

	// get envs and substitute with cloud args
	pEnvs := envs(p.Env, v.hypervisorInfo.envs)
	np.env = pEnvs

	// replace args cloud args as well plus existing envs
	pArgs, err := p.ProgramArgs()
	if err != nil {
		return err
	}
	np.args = args(pArgs[1:], np.env)

	np.path = calculatePath(pArgs[0], p)

	if len(np.path) == 0 {
		logError("application %s (%s) does not exist", pArgs[0], np.path)
		return fmt.Errorf("program %s can not be found", pArgs[0])
	}

	logDebug("launching %s", np.path)

	// run bootstrap functions
	np.bootstrap()

	logDebug("launch args %v", np.args)
	logDebug("launch envs %v", np.env)

	err = np.launch(v.user)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// that can be a missing binary or missing linker
			// let's try to make the error message better
			// if the binary exists it has to be a missing linker
			if _, err := os.Stat(np.path); err == nil {
				return fmt.Errorf("ld linker missing for %s", np.path)
			}

			return fmt.Errorf("%s application missing", np.path)

		}
		return err
	}

	// Register Program Terminate Wait Signal
	terminateSignals[np], err = p.Terminate.Signal()
	return err
}

func reapProcs(programs []*program) {

	pids := make(reap.PidCh, 1)
	errors := make(reap.ErrorCh, 1)

	go reap.ReapChildren(pids, errors, nil, nil)

	for {
		select {
		case pid := <-pids:
			logDebug("process %d finished", pid)
			for _, p := range programs {

				if p.cmd != nil && p.cmd.Process != nil && p.cmd.Process.Pid == pid {
					p.reaper = true
					break
				}

			}
			handleExit(programs)
		case err := <-errors:
			logError("error wait pid %s", err.Error())
		}
	}

}

// Launch starts all applications in vcfg
func (v *Vinitd) Launch() error {

	var wg sync.WaitGroup
	wg.Add(len(v.vcfg.Programs))

	logDebug("starting %d programs", len(v.vcfg.Programs))

	errors := make(chan error)
	wgDone := make(chan bool)

	go reapProcs(v.programs)

	go listenToProcesses(v.programs)

	for _, p := range v.programs {

		go func(p *program) {
			err := v.launchProgram(p)
			if err != nil {
				errors <- err
			}
			wg.Done()
		}(p)

	}

	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		break
	case err := <-errors:
		SystemPanic("starting program failed: %s", err.Error())
	}

	logDebug("all apps started")
	initStatus = statusLaunched

	return nil
}
