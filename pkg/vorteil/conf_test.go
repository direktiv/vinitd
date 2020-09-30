package vorteil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBootDisk(t *testing.T) {

	d, err := bootDisk()
	assert.NoError(t, err)
	assert.Equal(t, "/dev/sda", d)

}

func testLogFn(level LogLevel, format string, values ...interface{}) {
	fmt.Printf(format, values...)
}

func TestReadVCFGFile(t *testing.T) {

	v := New(testLogFn)
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
