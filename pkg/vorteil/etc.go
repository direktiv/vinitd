/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/rakyll/statik/fs"
)

const (
	pathMachineID = "/etc/machine-id"
)

var (
	etcFiles = []string{"group", "localtime", "nsswitch.conf", "passwd", "resolv.conf"}
)

func writeEtcFile(baseName, fullName string) error {

	fs, err := fs.New()
	if err != nil {
		return err
	}

	r, err := fs.Open(fmt.Sprintf("/%s.dat", baseName))
	if err != nil {
		return err
	}
	defer r.Close()

	contents, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(fullName, contents, 0644)
	if err != nil {
		return err
	}

	return nil
}

func generateEtcHosts(hostname string) {

	if _, err := os.Stat("/etc/hosts"); os.IsNotExist(err) {
		logDebug("file /etc/hosts does not exist, creating")

		var str strings.Builder
		str.WriteString("127.0.0.1\tlocalhost\n")
		str.WriteString(fmt.Sprintf("127.0.0.1\t%s\n", hostname))

		str.WriteString("::1\tip6-localhost ip6-loopback\n")
		str.WriteString("fe00::0\tip6-localnet\n")
		str.WriteString("ff00::0\tip6-mcastprefix\n")
		str.WriteString("ff02::1\tip6-allnodes\n")
		str.WriteString("ff02::2\tip6-allrouters\n")

		logDebug("/etc/hosts:\n%s", str.String())

		err = ioutil.WriteFile("/etc/hosts", []byte(str.String()), 0644)
		if err != nil {
			logError("can not create %s file: %v", "/etc/hosts", err)
		}
	}

}

func appendUserFile(k, v, user string) {

	// check if records exist for uid 1000
	e, err := ioutil.ReadFile(k)
	if err != nil {
		logError("can not read %s", k)
		return
	}

	if !strings.Contains(string(e), fmt.Sprintf("%s:x:1000", user)) {
		logDebug("append values to %s", k)
		e = append(e, fmt.Sprintf("%s\n", v)...)
		ioutil.WriteFile(k, e, 0644)
	}

}

func createUserFile(k, v, user string) {

	err := ioutil.WriteFile(k, []byte(fmt.Sprintf("%s\n", v)), 0644)
	if err != nil {
		logError(err.Error())
	}

}

func generateEtcMachineID() {

	if _, err := os.Stat(pathMachineID); os.IsNotExist(err) {
		id, err := uuid.NewRandom()
		if err != nil {
			logError(err.Error())
			return
		}

		err = ioutil.WriteFile(pathMachineID, []byte(id.String()), 0644)
		if err != nil {
			logError(err.Error())
			return
		}
	}

}

func addVorteilUserGroup(user string) {

	if user == "" {
		user = "vorteil"
	}

	etc := map[string]string{
		"/etc/passwd": fmt.Sprintf("root:x:0:0:root:/:/bin/false\n%s:x:1000:1000:%s:/:/bin/false", user, user),
		"/etc/group":  fmt.Sprintf("root:x:0:root\n%s:x:1000:%s", user, user),
	}

	for k, v := range etc {
		logDebug("checking %s", k)
		if _, err := os.Stat(k); err == nil {
			appendUserFile(k, v, user)
		} else if os.IsNotExist(err) {
			createUserFile(k, v, user)
		}
	}
}

/* etcGenerateFiles creates required files in /etc. The variable
   'base' is basically only for testing and should be '/etc' during runtime */
func etcGenerateFiles(hostname, user string) error {

	os.MkdirAll("/etc", 0755)

	addVorteilUserGroup(user)

	for _, f := range etcFiles {
		fullName := filepath.Join("/etc", f)
		if _, err := os.Stat(fullName); os.IsNotExist(err) {
			logDebug("creating file %s", fullName)
			writeEtcFile(f, fullName)
		}
	}

	// set hostname (/etc/hostname)
	f, err := os.OpenFile("/etc/hostname", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, strings.NewReader(hostname))
	if err != nil {
		return err
	}

	// generate /etc/hosts
	generateEtcHosts(hostname)

	generateEtcMachineID()

	return nil
}
