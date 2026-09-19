package sign

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/ncw/directio"
)

type (
	header struct {
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
)

const (
	// PageSize is the directio block size
	PageSize = directio.BlockSize

	// PageSizeInt64 is the int64 conversion of directio block size
	PageSizeInt64 = int64(directio.BlockSize) // Introduce a constant for int64 conversion of PageSize

	HBDiskSignature = "\x3d\xc1\x3c\x87\xc0\x5b\xe3\xb6"
	HBDiskVersion   = 1

	hbChecksumOffset = 0
	hbMagicOffset    = hbChecksumOffset + 4
	hbVersionOffset  = hbMagicOffset + len(HBDiskSignature)
	hbPageSizeOffset = hbVersionOffset + 4
	hbSlotSizeOffset = hbPageSizeOffset + 4
	hbUUIDOffset     = hbSlotSizeOffset + 4
	hbHeaderSize     = hbUUIDOffset + 16
)

func CreateAndFillDisk(path string) error {
	var hbCRC32CTable = crc32.MakeTable(crc32.Castagnoli)

	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek start: %s", err)
	}

	block := directio.AlignedBlock(PageSize)
	if len(block) < hbHeaderSize {
		return fmt.Errorf("block size %d is too small for osvcfs header", len(block))
	}

	copy(block[hbMagicOffset:], HBDiskSignature)
	binary.LittleEndian.PutUint32(block[hbVersionOffset:], HBDiskVersion)
	binary.LittleEndian.PutUint32(block[hbPageSizeOffset:], uint32(PageSize))
	binary.LittleEndian.PutUint32(block[hbSlotSizeOffset:], uint32(SlotSize))

	u := uuid.New()
	copy(block[hbUUIDOffset:], u[:])

	checksum := crc32.Checksum(block[hbMagicOffset:hbHeaderSize], hbCRC32CTable)

	binary.LittleEndian.PutUint32(block[hbChecksumOffset:], checksum)

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
		return fmt.Errorf("seek start: %w", err)
	}

	emptyBlock := directio.AlignedBlock(PageSize)

	if _, err := f.Write(emptyBlock); err != nil {
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
		return nil, fmt.Errorf("seek start: %w", err)
	}

	block := directio.AlignedBlock(PageSize)
	if _, err := io.ReadFull(f, block); err != nil {
		return nil, fmt.Errorf("read full: %w", err)
	}

	sig := make([]byte, len(HBDiskSignature))
	copy(sig, block[hbMagicOffset:hbMagicOffset+len(HBDiskSignature)])

	return sig, nil
}

func EnsureSignature(path string) (bool, error) {
	signature, err := getSignature(path)
	if err != nil {
		return false, err
	}
	return string(signature) == string(HBDiskSignature), nil
}
