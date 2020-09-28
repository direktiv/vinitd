package vorteil

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangeDiskScheduler(t *testing.T) {

	s, err := ioutil.ReadFile("/sys/block/sda/queue/scheduler")
	assert.NoError(t, err)
	assert.Equal(t, "[none] \n", string(s))

}

func TestSetupBasicDirectories(t *testing.T) {

	os.MkdirAll("/tmp/all", 0777)
	err := setupBasicDirectories("/tmp/all")
	assert.NoError(t, err)

	assert.DirExists(t, "/tmp/all/dev/pts")

	f, err := ioutil.ReadDir("/tmp/all/proc")
	assert.NoError(t, err)
	assert.True(t, len(f) > 0)

	f, err = ioutil.ReadDir("/tmp/all/sys")
	assert.NoError(t, err)
	assert.True(t, len(f) > 0)

}
