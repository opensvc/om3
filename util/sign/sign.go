package sign

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unsafe"

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

	HeaderSize = int64(unsafe.Sizeof(header{}) * 8)

	HBDiskSignature = "\x3d\xc1\x3c\x87\xc0\x5b\xe3\xb6"
	HBDiskVersion   = 3
)

func CreateAndFillDisk(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}
	headerSize := HeaderSize
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek start: %w", err)
	}

	block := directio.AlignedBlock(int(headerSize))
	copy(block[0:], HBDiskSignature)

	binary.LittleEndian.PutUint32(block[len(HBDiskSignature):], uint32(HBDiskVersion))

	binary.LittleEndian.PutUint32(block[len(HBDiskSignature)+4:], uint32(PageSize))

	binary.LittleEndian.PutUint32(block[len(HBDiskSignature)+8:], uint32(SlotSize))

	u := uuid.New()
	copy(block[len(HBDiskSignature)+12:], u[:])

	if _, err := f.Write(block); err != nil {
		return fmt.Errorf("write signature block: %w", err)
	}

	return nil
}

func RemoveHeaderFromDisk(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}
	headerSize := HeaderSize
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek start: %w", err)
	}

	emptyBlock := directio.AlignedBlock(int(headerSize))
	copy(emptyBlock, make([]byte, int(headerSize)))

	if _, err := f.Write(emptyBlock); err != nil {
		return fmt.Errorf("write empty block: %w", err)
	}
	return nil
}

func getSignature(path string) ([]byte, error) {
	_, err := os.Stat(path)
	if err != nil {
		return []byte{}, err
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return []byte{}, err
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return []byte{}, fmt.Errorf("seek start: %w", err)
	}

	block := directio.AlignedBlock(len(HBDiskSignature))
	if _, err := io.ReadFull(f, block); err != nil {
		return []byte{}, fmt.Errorf("read full: %w", err)
	}

	return block, nil
}

func EnsureSignature(path string) (bool, error) {
	signature, err := getSignature(path)
	if err != nil {
		return false, err
	}
	return string(signature) == HBDiskSignature, nil
}
