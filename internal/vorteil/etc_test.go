package vorteil

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testString = "vorteil"
)

func createEtcFile(base, file string) error {

	testString := []byte(testString)
	err := ioutil.WriteFile(filepath.Join(base, file), testString, 0644)
	if err != nil {
		return err
	}

	return nil
}

func md5File(filePath string) (string, error) {
	var returnMD5String string

	file, err := os.Open(filePath)
	if err != nil {
		return returnMD5String, err
	}

	defer file.Close()
	hash := md5.New()

	if _, err := io.Copy(hash, file); err != nil {
		return returnMD5String, err
	}
	return hex.EncodeToString(hash.Sum(nil)[:16]), nil
}

func TestEtcFilesExist(t *testing.T) {

	New(LogFnStdout)

	// create fake directories
	base, err := ioutil.TempDir(os.TempDir(), "fake_etc1")
	assert.NoError(t, err)
	defer os.Remove(base)

	// create all filename
	for _, f := range EtcFiles {
		err = createEtcFile(base, f)
		assert.NoError(t, err)
	}

	err = etcGenerateFiles(base, "vorteil", "vorteil")
	assert.NoError(t, err)

	// all files existed so they should have still the same size
	for _, f := range EtcFiles {
		fi, err := os.Stat(filepath.Join(base, f))
		assert.NoError(t, err)
		assert.Equal(t, fi.Size(), int64(len(testString)))
	}

	// check if hostname exists and has value
	fi, err := os.Stat(filepath.Join(base, "hostname"))
	assert.NoError(t, err)
	assert.Equal(t, fi.Size(), int64(len("vorteil")))

}

func TestEtcFilesNotExist(t *testing.T) {

	New(LogFnStdout)

	// create fake directories
	base, err := ioutil.TempDir(os.TempDir(), "fake_etc1")
	assert.NoError(t, err)
	defer os.Remove(base)

	err = etcGenerateFiles(base, "vorteil", "vorteil")
	assert.NoError(t, err)

	// compare the original etc file and new one as md5 hash
	for _, f := range EtcFiles {
		newFile := filepath.Join(base, f)
		origFile := fmt.Sprintf("../../build/etc/%s.dat", f)

		md5, err := md5File(newFile)
		assert.NoError(t, err)
		md5Comp, err := md5File(origFile)
		assert.NoError(t, err)

		assert.Equal(t, md5, md5Comp)
	}

}
