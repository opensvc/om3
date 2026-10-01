package sign

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"syscall"

	"github.com/google/uuid"
	"github.com/ncw/directio"
)

type (
	Header struct {
		Signature [8]byte
		Version   uint32
		BlockSize uint32
		SlotSize  uint32
		UUID      [16]byte
	}
)

var (
	// SlotSize is the data size reserved for a single node
	SlotSize = 1024 * 1024

	SlotSizeInt64 = int64(SlotSize)

	CRC32CTable = crc32.MakeTable(crc32.Castagnoli)

	ErrWrongSignature   = errors.New("wrong signature")
	ErrLegacySignature  = errors.New("legacy signature format detected")
	ErrChecksumMismatch = errors.New("checksum mismatch")
	ErrBlockTooSmall    = errors.New("block size too small for heartbeat disk header")
)

const (
	// PageSize is the directio block size
	PageSize = directio.BlockSize

	// PageSizeInt64 is the int64 conversion of directio block size
	PageSizeInt64 = int64(directio.BlockSize) // Introduce a constant for int64 conversion of PageSize

	HBDiskSignature = "\x3d\xc1\x3c\x87\xc0\x5b\xe3\xb6"
	HBDiskVersion   = 1

	HBCrcOffset      = 0
	HBMagicOffset    = HBCrcOffset + 4
	HBVersionOffset  = HBMagicOffset + len(HBDiskSignature)
	HBPageSizeOffset = HBVersionOffset + 4
	HBSlotSizeOffset = HBPageSizeOffset + 4
	HBUUIDOffset     = HBSlotSizeOffset + 4
	HBHeaderSize     = HBUUIDOffset + 16
)

func VerifyHeader(block []byte) error {
	if len(block) < HBHeaderSize {
		return fmt.Errorf("%s: %d", ErrBlockTooSmall, len(block))
	}
	if string(block[HBMagicOffset:HBMagicOffset+len(HBDiskSignature)]) == HBDiskSignature {
		checksum := crc32.Checksum(block[HBMagicOffset:HBHeaderSize], CRC32CTable)
		expectedChecksum := binary.LittleEndian.Uint32(block[HBCrcOffset:])
		if checksum != expectedChecksum {
			return ErrChecksumMismatch
		}
		return nil
	}
	if string(block[:len(HBDiskSignature)]) == HBDiskSignature {
		return ErrLegacySignature
	}
	return ErrWrongSignature
}

// CreateAndFillDisk writes the heartbeat header on the disk at path, and
// returns once the disk holds it.
func CreateAndFillDisk(path string) error {
	block := directio.AlignedBlock(PageSize)
	if len(block) < HBHeaderSize {
		return fmt.Errorf("block size %d is too small for heartbeat disk header", len(block))
	}

	copy(block[HBMagicOffset:], HBDiskSignature)
	binary.LittleEndian.PutUint32(block[HBVersionOffset:], HBDiskVersion)
	binary.LittleEndian.PutUint32(block[HBPageSizeOffset:], uint32(PageSize))
	binary.LittleEndian.PutUint32(block[HBSlotSizeOffset:], uint32(SlotSize))

	u := uuid.New()
	copy(block[HBUUIDOffset:], u[:])

	checksum := crc32.Checksum(block[HBMagicOffset:HBHeaderSize], CRC32CTable)
	binary.LittleEndian.PutUint32(block[HBCrcOffset:], checksum)

	if err := writeHeaderBlock(path, block); err != nil {
		return fmt.Errorf("write signature block: %w", err)
	}
	return nil
}

// writeHeaderBlock writes block at the start of the disk at path the way the
// heartbeats read it, with direct and synchronous i/o, and reads it back from
// the disk, so it returns once the disk holds it.
//
// It used to be written through the page cache, and the write returned before
// the disk held it. The heartbeats read the disk with direct i/o, and the
// peers read it from another node: a signature that did not reach the disk
// left them reading none, and failing to start, after a sign that said it was
// done.
//
// A filesystem refusing direct i/o, which no device does, is written to with
// synchronous i/o.
func writeHeaderBlock(path string, block []byte) error {
	f, err := directio.OpenFile(path, os.O_RDWR|os.O_SYNC, 0644)
	if errors.Is(err, syscall.EINVAL) {
		f, err = os.OpenFile(path, os.O_RDWR|os.O_SYNC, 0644)
	}
	if err != nil {
		return err
	}
	defer f.Close()

	n, err := f.WriteAt(block, 0)
	if err != nil {
		return err
	}
	if n != len(block) {
		return io.ErrShortWrite
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	readBack := directio.AlignedBlock(len(block))
	if _, err := f.ReadAt(readBack, 0); err != nil {
		return fmt.Errorf("read back: %w", err)
	}
	if !bytes.Equal(readBack, block) {
		return fmt.Errorf("%s does not hold the block written: the write did not reach the disk", path)
	}
	return nil
}

// RemoveHeaderFromDisk erases the heartbeat header of the disk at path, and
// returns once the disk no longer holds it.
func RemoveHeaderFromDisk(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	if err := writeHeaderBlock(path, directio.AlignedBlock(PageSize)); err != nil {
		return fmt.Errorf("write empty block: %w", err)
	}
	return nil
}

func getSignature(path string) ([]byte, error) {
	_, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek start: %s", err)
	}

	block := directio.AlignedBlock(PageSize)
	if _, err := io.ReadFull(f, block); err != nil {
		return nil, fmt.Errorf("read full: %s", err)
	}

	if string(block[HBMagicOffset:HBMagicOffset+len(HBDiskSignature)]) == HBDiskSignature {
		sig := make([]byte, len(HBDiskSignature))
		copy(sig, block[HBMagicOffset:HBMagicOffset+len(HBDiskSignature)])
		return sig, nil
	}
	if string(block[:len(HBDiskSignature)]) == HBDiskSignature {
		sig := make([]byte, len(HBDiskSignature))
		copy(sig, block[:len(HBDiskSignature)])
		return sig, nil
	}

	return nil, ErrWrongSignature
}

func EnsureSignature(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, fmt.Errorf("seek start: %s", err)
	}

	block := directio.AlignedBlock(PageSize)
	if _, err := io.ReadFull(f, block); err != nil {
		return false, fmt.Errorf("read full: %s", err)
	}

	if err := VerifyHeader(block); err != nil {
		if errors.Is(err, ErrLegacySignature) {
			return true, nil
		}
		return false, nil
	}
	return true, nil
}
