/**
 * SPDX-License-Identifier: Apache-2.0
 * Copyright 2020 vorteil.io Pty Ltd
 */

package vorteil

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"os"
)

type gptModifier struct {
	f    *os.File
	p    string
	size int64

	partitionEntry       *PartitionEntry
	gptHeader            *GPTHeader
	backupGPTHeader      *GPTHeader
	sectorsAdded         uint64
	originalBackupGPTLBA uint64
}

type newGPTModifierArgs struct {
	File *os.File
	Path string
}

func newGPTModifier(args *newGPTModifierArgs) (*gptModifier, error) {

	m := &gptModifier{
		f: args.File,
		p: args.Path,
	}

	var err error
	m.gptHeader, err = m.getGPTHeader()
	if err != nil {
		return nil, err
	}

	m.originalBackupGPTLBA = m.gptHeader.BackupLBA

	m.size, err = m.f.Seek(0, 2)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// AddPartition ..
func (m *gptModifier) addPartition() error {

	// pe := new(PartitionEntry)
	tableOffset := int64(SECTOR_SIZE * 2)
	var partitionEntry *PartitionEntry
	var peOffset int64

	var err error

	_, err = m.f.Seek(2*SECTOR_SIZE, 0)
	if err != nil {
		return err
	}

	peLen := 128
	for i := 0; i < 128; i++ { // can only be a max of 128 partition entries
		pe := new(PartitionEntry)
		err = binary.Read(m.f, binary.LittleEndian, pe)
		if err != nil {
			return err
		}

		if pe.FirstLBA == 0 {
			// no partition entry here - we've reached the end
			break
		}

		partitionEntry = pe
		peOffset = tableOffset + int64(i*peLen)
	}

	pe := new(PartitionEntry)
	pe.FirstLBA = partitionEntry.LastLBA + 1
	pe.LastLBA = m.lastUsableLBA()

	_, err = m.f.Seek(peOffset+int64(peLen), 0)
	if err != nil {
		return err
	}

	err = binary.Write(m.f, binary.LittleEndian, pe)
	if err != nil {
		return err
	}

	m.gptHeader.NoOfParts++
	return nil
}

// Grow ..
func (m *gptModifier) grow() error {

	if !m.needsResize() {
		return nil
	}

	var err error
	m.sectorsAdded, err = m.extendFinalPartition()
	if err != nil {
		return err
	}

	err = m.updatePrimaryGPTHeader()
	if err != nil {
		return err
	}

	err = m.UpdateSecondaryGPTHeader()
	if err != nil {
		return err
	}

	err = m.relocateRedundantGPTTable()
	if err != nil {
		return err
	}

	err = m.updateMBR()
	if err != nil {
		return err
	}

	return nil
}

// UpdateMBR ..
func (m *gptModifier) updateMBR() error {

	b := make([]byte, SECTOR_SIZE)
	_, err := m.f.ReadAt(b, 0)
	if err != nil {
		return err
	}

	mbr := new(ProtectiveMBREntry)
	err = binary.Read(bytes.NewReader(b), binary.LittleEndian, mbr)
	if err != nil {
		return err
	}

	mbr.NumberOfSectors = uint32(m.size/SECTOR_SIZE) - 1

	_, err = m.f.Seek(0, 0)
	if err != nil {
		return err
	}

	err = binary.Write(m.f, binary.LittleEndian, mbr)
	if err != nil {
		return err
	}

	return nil
}

// GetGPTHeader ..
func (m *gptModifier) getGPTHeader() (*GPTHeader, error) {

	_, err := m.f.Seek(SECTOR_SIZE, 0)
	if err != nil {
		return nil, err
	}

	primaryGPT := new(GPTHeader)
	err = binary.Read(m.f, binary.LittleEndian, primaryGPT)
	if err != nil {
		return nil, err
	}

	return primaryGPT, nil
}

func (m *gptModifier) lastUsableLBA() uint64 {
	return uint64(m.size/SECTOR_SIZE - 34)
}

// UpdatePrimaryGPTHeader ..
func (m *gptModifier) updatePrimaryGPTHeader() error {

	// fields to update: backupLBA, lastUsableLBA, crc, crcparts
	// lastUsableLBA should equal totalSize/512 - 33

	m.gptHeader.LastUsableLBA = m.lastUsableLBA()
	m.gptHeader.BackupLBA = uint64(m.size/SECTOR_SIZE) - 1
	err := m.calculateCRCs(m.gptHeader, false)
	if err != nil {
		return err
	}

	_, err = m.f.Seek(1*SECTOR_SIZE, 0)
	if err != nil {
		return err
	}

	err = binary.Write(m.f, binary.LittleEndian, m.gptHeader)
	if err != nil {
		return err
	}

	return nil
}

// UpdateSecondaryGPTHeader ..
func (m *gptModifier) UpdateSecondaryGPTHeader() error {

	_, err := m.f.Seek(1*SECTOR_SIZE, 0)
	if err != nil {
		return err
	}

	g := new(GPTHeader)
	err = binary.Read(m.f, binary.LittleEndian, g)
	if err != nil {
		return err
	}

	g.CurrentLBA = m.gptHeader.BackupLBA
	g.BackupLBA = m.gptHeader.CurrentLBA

	g.StartLBAParts = g.CurrentLBA - 32

	err = m.calculateCRCs(g, true)
	if err != nil {
		return err
	}

	_, err = m.f.Seek(int64(g.CurrentLBA*SECTOR_SIZE), 0)
	if err != nil {
		return err
	}

	err = binary.Write(m.f, binary.LittleEndian, g)
	if err != nil {
		return err
	}

	m.backupGPTHeader = g
	return nil
}

func (m *gptModifier) calculateCRCs(g *GPTHeader, skipParts bool) error {

	// zero the crc field in preparation
	g.Crc = 0

	// calc crc of partition entries array
	if !skipParts {
		pea := make([]byte, SECTOR_SIZE*32)
		_, err := m.f.ReadAt(pea, int64(g.StartLBAParts*SECTOR_SIZE))
		if err != nil {
			return err
		}

		crc := crc32.NewIEEE()
		_, err = crc.Write(pea)
		if err != nil {
			return err
		}

		g.CrcParts = crc.Sum32()
	} else {
		g.CrcParts = m.gptHeader.CrcParts
	}

	// calc header crc val
	hdrBuf := new(bytes.Buffer)
	err := binary.Write(hdrBuf, binary.LittleEndian, g)
	if err != nil {
		return err
	}

	crc := crc32.NewIEEE()
	_, err = crc.Write(hdrBuf.Bytes()[:g.HeaderSize])
	if err != nil {
		return err
	}

	g.Crc = crc.Sum32()
	return nil
}

// NeedsResize ..
func (m *gptModifier) needsResize() bool {
	return (m.gptHeader.LastUsableLBA+35)*SECTOR_SIZE < uint64(m.size)
}

// ExtendFinalPartition .. returns number of sectors the partition was grown by
func (m *gptModifier) extendFinalPartition() (uint64, error) {

	// get last present partition entry from table
	tableOffset := int64(SECTOR_SIZE * 2)
	_, err := m.f.Seek(tableOffset, 0)
	if err != nil {
		return 0, err
	}

	// var partitionEntry *PartitionEntry
	var peOffset int64

	peLen := 128
	for i := 0; i < 128; i++ { // can only be a max of 128 partition entries
		pe := new(PartitionEntry)
		err = binary.Read(m.f, binary.LittleEndian, pe)
		if err != nil {
			return 0, err
		}

		if pe.FirstLBA == 0 {
			// no partition entry here - we've reached the end
			break
		}

		m.partitionEntry = pe
		peOffset = tableOffset + int64(i*peLen)
	}

	// adjust values in pe and re-write to disk
	out := m.lastUsableLBA() - m.partitionEntry.LastLBA

	m.partitionEntry.LastLBA = m.lastUsableLBA()
	_, err = m.f.Seek(peOffset, 0)
	if err != nil {
		return 0, err
	}

	err = binary.Write(m.f, binary.LittleEndian, m.partitionEntry)
	if err != nil {
		return 0, err
	}

	return out, nil
}

func (m *gptModifier) relocateRedundantGPTTable() error {

	// originalLocation := int(m.GPTHeader.StartLBAParts * SECTOR_SIZE)
	redundantGPTOffset := (m.originalBackupGPTLBA - 32) * SECTOR_SIZE
	gptLen := 33 * SECTOR_SIZE

	// overwrite previous redundant GPT location with zeroes to free it up
	_, err := m.f.Seek(int64(redundantGPTOffset), 0)
	if err != nil {
		return err
	}

	_, err = io.CopyN(m.f, zeroes, int64(gptLen))
	if err != nil {
		return err
	}

	gptOff := m.backupGPTHeader.StartLBAParts * SECTOR_SIZE

	// then table
	for i := 0; i < 32; i++ {

		b := make([]byte, SECTOR_SIZE)

		_, err = m.f.ReadAt(b, int64((m.gptHeader.StartLBAParts*SECTOR_SIZE)+uint64(i*SECTOR_SIZE)))
		if err != nil {
			return err
		}

		_, err = m.f.WriteAt(b, int64(gptOff+uint64(i*SECTOR_SIZE)))
		if err != nil {
			return err
		}
	}

	return nil
}

// Constants ...
const (
	SECTOR_SIZE          = 512
	PARTITION_ENTRY_SIZE = 128
)

var (
	newBackupGPTLocation int64
	gpt1, gpt2           *GPTHeader
	totalSectors         uint32
)
