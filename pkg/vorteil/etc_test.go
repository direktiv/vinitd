package vorteil

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testString = "vorteil"
)

func TestEtcFiles(t *testing.T) {

	// this is to protect the removeall /etc
	err := New(testLogFn).readVCFG("/dev/sda2")
	if err != nil {
		t.Logf("not running in a VM")
	}

	// remove all
	os.RemoveAll("/etc")

	err = etcGenerateFiles(testString, testString)
	assert.NoError(t, err)

	allFiles := append(etcFiles, "hostname", "hosts")

	// all the files should exist now
	for _, f := range allFiles {
		_, err := os.Stat(filepath.Join("/etc", f))
		assert.NoError(t, err)
	}

	r, _ := ioutil.ReadDir("/etc")
	assert.Equal(t, len(r), len(allFiles))

	h, _ := ioutil.ReadFile("/etc/hostname")

	assert.Equal(t, string(h), testString)

	// now test user add
	// empty string uses vorteil user
	u, _ := ioutil.ReadFile("/etc/passwd")
	g, _ := ioutil.ReadFile("/etc/group")

	// has been created with user vorteil befroe so should be same size
	addVorteilUserGroup("")

	ua, _ := ioutil.ReadFile("/etc/passwd")
	ga, _ := ioutil.ReadFile("/etc/group")

	assert.Equal(t, u, ua)
	assert.Equal(t, g, ga)

	// this should add a user
	addVorteilUserGroup("random")

	ua, _ = ioutil.ReadFile("/etc/passwd")
	ga, _ = ioutil.ReadFile("/etc/group")

	assert.NotEqual(t, u, ua)
	assert.NotEqual(t, g, ga)

}
