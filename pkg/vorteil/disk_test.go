package vorteil

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangeDiskScheduler(t *testing.T) {

	s, err := ioutil.ReadFile("/sys/block/%s/queue/scheduler")
	assert.Error(t, err)

	t.Logf("HH %v", string(s))

}
