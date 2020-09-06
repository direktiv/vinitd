package vorteil

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vorteil/vorteil/pkg/vcfg"
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

func calculatePath(p vcfg.Program) string {

	// nothing to caluclate if absolute
	if path.IsAbs(p.Binary) {
		return p.Binary
	}

	// if that file exists we return
	if _, err := os.Stat(filepath.Join(p.Cwd, p.Binary)); err == nil {
		// if that file exists we return the absolute path
		// if there is an error we are trying to just add a leading /
		path, err := filepath.Abs(filepath.Join(p.Cwd, p.Binary))
		if err != nil {
			logError("can not create path for %s, err %v", p.Binary, err)
			return fmt.Sprintf("/%s", p.Binary)
		}
		return path
	}

	// last chance: maybe it is in PATH
	pathEnv := strings.Split(pickFromEnv(pathEnvName, p), pathSeperator)
	if len(pathEnv) > 0 {
		for _, c := range pathEnv {
			if _, err := os.Stat(filepath.Join(c, p.Binary)); err == nil {
				return filepath.Join(c, p.Binary)
			}
		}
	}

	return ""
}

// func (p *vcfg.Program) launch() error {

// strace override
// if p.strace == 0x1 {
// 	p.args.values = append(p.args.values, "")
// 	copy(p.args.values[1:], p.args.values)
// 	p.args.values[0] = p.fpath
// 	p.fpath = "/vorteil/strace"
// }
//
// cmd := exec.Command(p.fpath, p.args.values...)
// cmd.Env = p.env.values
// cmd.Dir = p.cwd
//
// var user string
// var rid int
//
// switch p.privilege {
// case 0: // root privilege
// 	rid = rootID
// 	user = "root"
// case 1: // superuser privilege
// 	user = fmt.Sprintf("%s (superuser)", p.vinitd.user)
// 	rid = userID
// default: // user privilege
// 	user = p.vinitd.user
// 	rid = userID
// }
//
// // either root or uid 1000
// cmd.SysProcAttr = &syscall.SysProcAttr{
// 	Credential: &syscall.Credential{Uid: uint32(rid), Gid: uint32(rid)},
// }
//
// if p.privilege == 1 {
// 	cmd.SysProcAttr.AmbientCaps = []uintptr{
// 		unix.CAP_CHOWN,
// 		unix.CAP_DAC_OVERRIDE,
// 		unix.CAP_DAC_READ_SEARCH,
// 		unix.CAP_FOWNER,
// 		unix.CAP_IPC_OWNER,
// 		unix.CAP_NET_ADMIN,
// 		unix.CAP_MKNOD,
// 		unix.CAP_NET_BIND_SERVICE,
// 		unix.CAP_NET_RAW,
// 		unix.CAP_SYS_ADMIN,
// 	}
// }
//
// logDebug("starting as %s, uid %d", user, rid)
//
// // Create stderr dir if it does not exists
// if _, err := os.Stat(filepath.Dir(p.stderr)); os.IsNotExist(err) {
// 	os.MkdirAll(filepath.Dir(p.stderr), 0)
// }
//
// stderr, err := os.OpenFile(p.stderr, os.O_WRONLY|os.O_APPEND, 0)
// if err != nil {
// 	return err
// }
//
// // Create stdout dir if it does not exists
// if _, err := os.Stat(filepath.Dir(p.stdout)); os.IsNotExist(err) {
// 	os.MkdirAll(filepath.Dir(p.stdout), 0)
// }
//
// stdout, err := os.OpenFile(p.stdout, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0)
// if err != nil {
// 	return err
// }
//
// cmd.Stderr = stderr
// cmd.Stdout = stdout
//
// err = cmd.Start()
// if err != nil {
// 	return err
// }
//
// p.status = STATUS_RUN
//
// return nil
// }

// bootstrapWaitForFile hangs process until the file appears, times out after 30 seconds.
func bootstrapWaitForFile(b *bootstrapInstruction, p *program) {
	// check length of args to see atleast one path
	if len(b.args) != 1 {
		logWarn("bootstrap 'WAIT_FILE' needs one value")
		return
	}

	// Loop through and stat the path to see when it exists
	count := 0
	for {
		if _, err := os.Stat(b.args[0]); err == nil {
			break
		}
		// Check count log a warning saying file hasn't appeared in 30 seconds repeat action
		if count%30 == 0 {
			logWarn("bootstrap 'WAIT_FILE' file %s hasn't appeared yet", b.args[0])
		}
		count++
		time.Sleep(time.Second * 1)
	}

}

// bootstrapWaitForPort hands process until the ports appear for certain network types, timesout after 30 seconds
func bootstrapWaitForPort(b *bootstrapInstruction, p *program) {

	// b.args[0] should be ifce, b.args[1:] should be all network
	if len(b.args) < 2 {
		logWarn("bootstrap 'WAIT_PORT' needs at least one value to listen out for")
		return
	}

	// interface to look on should always exist if they provide it if not its default eth0
	ief := strings.Split(b.args[0], "=")[1]
	ifce, err := net.InterfaceByName(ief)
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

	var ip string
	var isIpv4 bool
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
	for _, arg := range strings.Split(strings.TrimSpace(strings.Join(b.args[1:], " ")), " ") {
		// listen for ports to be alive
		wg.Add(1)
		// go routine it to test all ports currently asked for
		go listenForPort(fmt.Sprintf("%s:%s", ip, arg), &wg)
	}
	// wait till group resolves
	wg.Wait()
}

// listenForPort listens for port in a loop returns nothing but resolves wait group when function succesfully dials port
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

func bootstrapNotdefined(b *bootstrapInstruction, p *program) string {

	// if this is not a pair, we ignore
	if len(b.args) != 2 {
		// p.vinitd.Logging(LOG_WARNING, "bootstrap 'DEFINE_IF_NOT_DEFINED' needs two values")
		return ""
	}

	// check if it has been set
	for _, val := range p.env.values {
		if strings.HasPrefix(val, fmt.Sprintf("%s=", b.args[0])) {
			return ""
		}
	}

	for k, val := range p.vinitd.hypervisorInfo.envs {
		b.args[1] = strings.ReplaceAll(b.args[1], fmt.Sprintf(replaceString, k), val)
	}

	return fmt.Sprintf(environString, b.args[0], b.args[1])

}

func bootstrapReplace(b *bootstrapInstruction, p *program) {

	if len(b.args) != 3 {
		logWarn("bootstrap 'FIND_AND_REPLACE' needs two values")
		return
	}

	m := make(map[string]string)
	for _, a := range b.args {
		aa := strings.SplitN(a, "=", 2)
		m[aa[0]] = aa[1]
	}

	txt, err := ioutil.ReadFile(m["file"])
	if err != nil {
		logWarn("file %s does does not exist to replace text", m["file"])
		return
	}

	// check if it has been set
	for _, val := range p.env.values {
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

	for _, b := range p.bootstraps {
		switch b.btype {
		case BOOTSTRAP_SLEEP:
			{
				time.Sleep(time.Duration(b.time) * time.Millisecond)
				break
			}
		case BOOTSTRAP_WAIT_FILE:
			{
				bootstrapWaitForFile(b, p)
				break
			}
		case BOOTSTRAP_WAIT_PORT:
			{
				bootstrapWaitForPort(b, p)
				break
			}
		case BOOTSTRAP_FIND_AND_REPLACE:
			{
				bootstrapReplace(b, p)
				break
			}
		case BOOTSTRAP_DEFINE_IF_NOT_DEFINED:
			{
				s := bootstrapNotdefined(b, p)
				if len(s) > 0 {
					p.env.count++
					p.env.values = append(p.env.values, s)
				}
				break
			}
		default:
			{
				logError("unknown bootstrap command: %d", b.btype)
			}
		}
		// p.vinitd.logAlways("BOOTST %v", b)
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

func (v *Vinitd) launchProgram(p vcfg.Program) error {

	fpath := calculatePath(p)

	if len(fpath) == 0 {
		logError("application %s (%s) does not exist", p.Binary, fpath)
		return fmt.Errorf("program %s can not be found", p.Binary)
	}

	// we can add the program to the list now
	np := &program{
		path:     fpath,
		vcfgProg: p,
	}

	v.programs = append(v.programs, np)
	logAlways("launching %s", fpath)

	logAlways("launching2 %v", p.Args)

	// get envs and substitue with cloud args
	pEnvs := envs(p.Env, v.hypervisorInfo.envs)
	np.env.values = pEnvs
	// np.env.count = len(pEnvs)

	logDebug("launching2 %v", p.Args)

	// replace args cloud args as well plus existing envs
	// np.args.values = args(p.Args[1:], pEnvs)
	// np.args.count =

	// run bootstrap functions
	// TODO bootstrap
	// p.bootstrap()

	// err := p.launch()
	// if err != nil {
	// 	if errors.Is(err, os.ErrNotExist) {
	// 		// that can be a missing binary or missing linker
	// 		// let's try to make the error message better
	// 		// if the binary exists it has to be a missing linker
	// 		if _, err := os.Stat(fpath); err == nil {
	// 			return fmt.Errorf("ld linker missing for %s", fpath)
	// 		}
	//
	// 		return fmt.Errorf("%s application missing", fpath)
	//
	// 	}
	// 	return err
	// }

	return nil
}

func (v *Vinitd) Launch() error {

	// TODO: start
	var wg sync.WaitGroup
	wg.Add(len(v.vcfg.Programs))

	logAlways("starting %d programs", len(v.vcfg.Programs))

	errors := make(chan error)
	wgDone := make(chan bool)

	// TODO: listen
	// go listenToProcesses(v.programs)

	for _, p := range v.vcfg.Programs {

		go func(p vcfg.Program) {
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

	return nil
}
