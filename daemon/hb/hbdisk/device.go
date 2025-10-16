package hbdisk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ncw/directio"
	"github.com/opensvc/om3/util/sign"

	"github.com/opensvc/om3/util/file"
)

type (
	device struct {
		mode string
		path string
		file *os.File

		// metaSize is the size of the header reserved on dev to store the
		// slot allocations. It is used by calculateDataSlotOffset
		metaSize int64
	}
)

const (
	// openDevicePermission is the permission used for open device file
	openDevicePermission = 0755

	// modeDirectIO is the mode for Linux Direct IO
	modeDirectIO = "directio"

	// modeRaw is the mode for non-Linux systems
	modeRaw = "raw" // Mode for non-Linux systems
)

// calculateMetaSlotOffset returns the offset of the meta page of the slot.
func (t *device) calculateMetaSlotOffset(slot int) int64 {
	return sign.PageSizeInt64 * int64(slot)
}

func (t *device) readSignature() (string, error) {
	if _, err := t.file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek: %w", err)
	}
	block := directio.AlignedBlock(sign.PageSize)
	if n, err := io.ReadAtLeast(t.file, block, len(sign.HBDiskSignature)); err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	} else if n < len(sign.HBDiskSignature) {
		return "", fmt.Errorf("expected %d bytes, got %d", len(sign.HBDiskSignature), n)
	} else if block[0] == endOfDataMarker {
		return "", fmt.Errorf("no data")
	}
	if _, err := t.file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek: %w", err)
	}
	return string(block[:len(sign.HBDiskSignature)]), nil
}

func (t *device) readMetaSlot(slot int) ([]byte, error) {
	offset := t.calculateMetaSlotOffset(slot)
	if _, err := t.file.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek offset %d: %w", offset, err)
	}
	block := directio.AlignedBlock(sign.PageSize)
	if _, err := io.ReadFull(t.file, block); err != nil {
		return nil, fmt.Errorf("read full at offset %d: %w", offset, err)
	}
	return block, nil
}

func (t *device) writeMetaSlot(slot int, b []byte) error {
	if len(b) > sign.PageSize {
		return fmt.Errorf("attempt to write too long data in meta slot %d", slot)
	}
	offset := t.calculateMetaSlotOffset(slot)
	if _, err := t.file.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("seek offset %d: %w", offset, err)
	}
	block := directio.AlignedBlock(sign.PageSize)
	copy(block, b)
	if _, err := t.file.Write(block); err != nil {
		return fmt.Errorf("write at offset %d: %w", offset, err)
	}
	return nil
}

// calculateDataSlotOffset calculates the byte offset of a data slot within the storage device.
func (t *device) calculateDataSlotOffset(slot int) int64 {
	return t.metaSize + sign.SlotSizeInt64*int64(slot)
}

func (t *device) readDataSlot(slot int) (capsule, error) {
	c := capsule{}
	offset := t.calculateDataSlotOffset(slot)
	if _, err := t.file.Seek(offset, io.SeekStart); err != nil {
		return c, fmt.Errorf("seek offset %d: %w", offset, err)
	}
	data := make([]byte, 0)
	totalRead := 0
	for {
		block := directio.AlignedBlock(sign.PageSize)
		n, err := io.ReadFull(t.file, block)
		totalRead += n
		if err != nil {
			return c, fmt.Errorf("read full at offset %d: %w", offset, err)
		}
		if n == 0 {
			break
		}
		i := bytes.IndexRune(block, endOfDataMarker)
		if i < 0 {
			data = append(data, block...)
		} else {
			data = append(data, block[:i]...)
			break
		}
		if totalRead >= sign.SlotSize {
			break
		}
	}
	if len(data) == 0 {
		return c, fmt.Errorf("read no data at offset %d", offset)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("unmarshall from offset %d data len %d from total read %d: %w", slot, len(data), totalRead, err)
	}
	return c, nil
}

func (t *device) writeDataSlot(slot int, b []byte) error {
	c := capsule{
		Msg:     b,
		Updated: time.Now(),
	}
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("msg encapsulation: %w", err)
	}
	b = append(b, []byte{endOfDataMarker}...)
	if len(b) > sign.SlotSize {
		return fmt.Errorf("attempt to write too long data in data slot %d", slot)
	}
	offset := t.calculateDataSlotOffset(slot)
	if _, err := t.file.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("seek offset %d: %w", offset, err)
	}
	remaining := len(b)
	for {
		if remaining == 0 {
			break
		}
		block := directio.AlignedBlock(sign.PageSize)
		copied := copy(block, b)
		if _, err := t.file.Write(block); err != nil {
			return fmt.Errorf("write at offset %d: %w", offset, err)
		}
		if copied < sign.PageSize {
			return nil
		}
		b = b[copied:]
		remaining -= copied
	}
	return nil
}

func (t *device) open() error {
	if t.path == "" {
		return fmt.Errorf("the 'dev' keyword is not set")
	}

	newDev, err := validateDevice(t.path)
	if err != nil {
		return err
	}

	if err := t.openDevice(newDev); err != nil {
		return err
	}
	if err := t.ensureHBSignature(); err != nil {
		return err
	}
	return nil
}

// ensureBlockDevice checks if the specified device path refers to a valid
// block device and returns an error if it is not.
func (t *device) ensureBlockDevice(path string) error {
	if ok, err := file.IsBlockDevice(path); err != nil {
		return fmt.Errorf("%s must be a block device: %w", path, err)
	} else if !ok {
		return fmt.Errorf("%s must be a block device", path)
	}
	return nil
}

// ensureBlockDevice checks if the specified device path refers to a valid
// char device and returns an error if it is not.
func (t *device) ensureCharDevice(path string) error {
	if ok, err := file.IsCharDevice(path); err != nil {
		return fmt.Errorf("%s must be a char device: %w", path, err)
	} else if !ok {
		return fmt.Errorf("%s must be a char device", path)
	}
	return nil
}

// ensureHBSignature verifies that the device contains the expected HB signature
// and returns an error if validation fails.
func (t *device) ensureHBSignature() error {
	if signature, err := t.readSignature(); err != nil {
		return fmt.Errorf("expected signature '%s' at offset 0: read failed: %w", sign.HBDiskSignature, err)
	} else if signature != sign.HBDiskSignature {
		return fmt.Errorf("expected signature '%s' at offset 0: found '%s'", sign.HBDiskSignature, signature)
	}
	return nil
}

// validateDevice resolves symlinks for the given path and ensures the target
// device exists, returning its resolved path.
// Returns an error if the path is invalid, the device does not exist,
// or other filesystem issues occur.
func validateDevice(path string) (string, error) {
	newPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("%s eval symlink: %w", path, err)
	}
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		return "", fmt.Errorf("%s does not exist: %w", path, err)
	} else if err != nil {
		return "", err
	}
	return newPath, nil
}
