package sign

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"

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

func CreateAndFillDisk(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek start: %s", err)
	}

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

	n, err := f.Write(block)
	if err != nil {
		return fmt.Errorf("write signature block: %s", err)
	}
	if n != len(block) {
		return io.ErrShortWrite
	}

	return nil
}

func RemoveHeaderFromDisk(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek start: %s", err)
	}

	emptyBlock := directio.AlignedBlock(PageSize)

	if _, err := f.Write(emptyBlock); err != nil {
		return fmt.Errorf("write empty block: %s", err)
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
