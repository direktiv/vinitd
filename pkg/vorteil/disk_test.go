package vorteil

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangeDiskScheduler(t *testing.T) {

	s, err := ioutil.ReadFile("/sys/block/sda/queue/scheduler")
	assert.NoError(t, err)

	t.Logf("HH %v", string(s))

}
