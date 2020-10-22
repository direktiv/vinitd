package vorteil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBootDisk(t *testing.T) {

	d, err := bootDisk()
	assert.NoError(t, err)
	assert.Equal(t, "/dev/sda", d)

}

func TestReadVCFGFile(t *testing.T) {

	v := New()
	err := v.readVCFG("/hw.raw")
	assert.NoError(t, err)

}

func TestOpenVCFGFile(t *testing.T) {

	_, err := openVCFGFile("does/not/exist")
	assert.Error(t, err)

	d, _ := bootDisk()
	_, err = openVCFGFile(d)

	assert.NoError(t, err)

}
