package vorteil

import (
	"os"
	"reflect"
)

// Format variables are used to identify fs formats
type Format string

// FSType vars ...
var (
	Ext2FS    = Format("ext2")
	Ext4FS    = Format("ext4")
	XFS       = Format("xfs")
	UnknownFS = Format("unknown")
)

var zeroes = &zr{}

var (
	ext2Signature = []byte{0x53, 0xEF}
	xfsSignature  = []byte{0x58, 0x46, 0x53, 0x42}
)

// detectFormat inspects the filesystem located on the file at 'path' that begins
// at offset 'start' (whence 0), and returns the determined filesystem format
func detectFormat(f *os.File, path string, start int64) (Format, error) {

	// check ext2 format
	off := start + 1080 // ext2 magic number exists at offset 56 of the superblock, which begins at offset 1024 from fs start
	b := make([]byte, 2)

	_, err := f.ReadAt(b, off)
	if err != nil {
		return UnknownFS, err
	}

	// do the bytes read from disk match the expected ext2 signature?
	if reflect.DeepEqual(b, ext2Signature) {
		return Ext2FS, nil
	}

	// check xfs format
	off = start
	b = make([]byte, 4)
	_, err = f.ReadAt(b, off)
	if err != nil {
		return UnknownFS, err
	}

	if reflect.DeepEqual(b, xfsSignature) {
		return XFS, nil
	}

	return UnknownFS, nil
}

// divide, rounding up
func ceiling(x, y int) int {
	return (x + y - 1) / y
}

type zr struct {
}

func (rdr *zr) Read(p []byte) (n int, err error) {

	if len(p) == 0 {
		return
	}
	p[0] = 0
	for bp := 1; bp < len(p); bp *= 2 {
		copy(p[bp:], p[:bp])
	}

	return len(p), nil
}
