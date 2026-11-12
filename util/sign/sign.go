package sign

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"strings"
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
		expectedChecksum := binary.LittleEndian.Uint32(block[HBCrcOffset:])
		binary.LittleEndian.PutUint32(block[HBCrcOffset:], 0)
		checksum := crc32.Checksum(block[:HBHeaderSize], CRC32CTable)
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

// SignOptions tells Sign how to sign a disk.
type SignOptions struct {
	// MaxSlots is the number of pages of the metadata area of the disk,
	// the header the first, which the slots of the nodes follow: the
	// max_slots of the heartbeat.
	MaxSlots int

	// MinSize is the size a disk smaller than is refused, zero for no
	// check: the metadata area and a data slot per node, plus the unused
	// slot 0.
	MinSize int64

	// Force signs a disk holding a signature, as a filesystem or a lvm
	// physical volume, that is not of a heartbeat.
	Force bool
}

var (
	// ErrForeignSignature is the error of a sign refused on a disk
	// holding a signature not of a heartbeat.
	ErrForeignSignature = errors.New("the disk holds a signature not of a heartbeat")

	// ErrTooSmall is the error of a sign refused on a disk too small for
	// the slots of the heartbeat.
	ErrTooSmall = errors.New("the disk is too small for the heartbeat")

	// probeSignature returns the signature a disk holds, as "TYPE=ext4",
	// empty for none: a variable, for the tests.
	probeSignature = blkidProbe
)

// MinSize returns the size of a disk heartbeat of maxSlots slots for nodes
// nodes: the metadata area, a page per slot, then a data slot per node after
// the unused slot 0.
func MinSize(maxSlots, nodes int) int64 {
	return int64(maxSlots)*PageSizeInt64 + int64(nodes+1)*SlotSizeInt64
}

// Sign writes the heartbeat header on the disk at path, and returns once the
// disk holds it.
//
// A disk with no heartbeat header, never a heartbeat disk or wiped, gets its
// metadata area zeroed first: the nodes take a page whose first byte is zero
// as a free slot, and on a disk holding what it held before, as a filesystem,
// they claimed slots past the end of the disk and never beat. Such a disk
// holding a signature, as a filesystem, is refused unless forced, as the sign
// erases opts.MaxSlots pages of it.
//
// A disk with a heartbeat header, valid, of the earlier format, or damaged,
// keeps its metadata: the slots of the nodes, and of the other clusters
// sharing it.
func Sign(path string, opts SignOptions) error {
	f, err := openSync(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	size := deviceSize(f)
	if opts.MinSize > 0 && size > 0 && size < opts.MinSize {
		return fmt.Errorf("%w: %s is %d bytes, under the %d bytes of %d metadata pages and a %d bytes data slot per node", ErrTooSmall, path, size, opts.MinSize, opts.MaxSlots, SlotSize)
	}

	current := directio.AlignedBlock(PageSize)
	if _, err := f.ReadAt(current, 0); err != nil {
		return fmt.Errorf("read the header: %w", err)
	}
	if !isHeartbeatHeader(current) {
		if !opts.Force {
			kind, err := probeSignature(path)
			if err != nil {
				return err
			}
			if kind != "" {
				return fmt.Errorf("%w: %s holds %s, which the sign would erase", ErrForeignSignature, path, kind)
			}
		}
		if err := zeroPages(f, 1, opts.MaxSlots, size); err != nil {
			return fmt.Errorf("zero the metadata area: %w", err)
		}
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

	binary.LittleEndian.PutUint32(block[HBCrcOffset:], 0)
	checksum := crc32.Checksum(block[:HBHeaderSize], CRC32CTable)
	binary.LittleEndian.PutUint32(block[HBCrcOffset:], checksum)

	if err := writeHeaderBlock(f, path, block); err != nil {
		return fmt.Errorf("write signature block: %w", err)
	}
	return nil
}

// isHeartbeatHeader says whether block holds a heartbeat header, valid, of
// the earlier format, or damaged: the disk is a heartbeat disk.
func isHeartbeatHeader(block []byte) bool {
	b := make([]byte, len(block))
	copy(b, block)
	err := VerifyHeader(b)
	return err == nil || errors.Is(err, ErrLegacySignature) || errors.Is(err, ErrChecksumMismatch)
}

// openSync opens the disk at path for direct and synchronous i/o, the way the
// heartbeats read it, or synchronous only on a filesystem refusing direct
// i/o, which no device does.
func openSync(path string) (*os.File, error) {
	f, err := directio.OpenFile(path, os.O_RDWR|os.O_SYNC, 0644)
	if errors.Is(err, syscall.EINVAL) {
		f, err = os.OpenFile(path, os.O_RDWR|os.O_SYNC, 0644)
	}
	return f, err
}

// deviceSize returns the size of the disk f, zero when it can not say.
func deviceSize(f *os.File) int64 {
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil || size < 0 {
		return 0
	}
	return size
}

// zeroPages writes zeroes over the pages from to to of the disk f, not past
// its size when known.
func zeroPages(f *os.File, from, to int, size int64) error {
	begin := int64(from) * PageSizeInt64
	end := int64(to) * PageSizeInt64
	if size > 0 && end > size {
		end = size
	}
	if begin >= end {
		return nil
	}
	chunk := directio.AlignedBlock(256 * PageSize)
	for offset := begin; offset < end; offset += int64(len(chunk)) {
		b := chunk
		if rest := end - offset; rest < int64(len(b)) {
			b = b[:rest]
		}
		if _, err := f.WriteAt(b, offset); err != nil {
			return err
		}
	}
	return f.Sync()
}

// blkidProbe returns the signature blkid finds on the disk at path, as
// "TYPE=ext4" or "PTTYPE=gpt", empty when it finds none or is not installed.
func blkidProbe(path string) (string, error) {
	blkid, err := exec.LookPath("blkid")
	if err != nil {
		return "", nil
	}
	out, err := exec.Command(blkid, "-p", "-o", "export", path).Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		// nothing found
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("probe the signatures of %s: %w", path, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "TYPE=") || strings.HasPrefix(line, "PTTYPE=") {
			return line, nil
		}
	}
	return "", nil
}

// writeHeaderBlock writes block at the start of the disk f at path the way
// the heartbeats read it, with direct and synchronous i/o, and reads it back
// from the disk, so it returns once the disk holds it.
//
// It used to be written through the page cache, and the write returned before
// the disk held it. The heartbeats read the disk with direct i/o, and the
// peers read it from another node: a signature that did not reach the disk
// left them reading none, and failing to start, after a sign that said it was
// done.
func writeHeaderBlock(f *os.File, path string, block []byte) error {
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

// Wipe erases the heartbeat header of the disk at path, then its metadata
// area of maxSlots pages, and returns once the disk no longer holds them.
//
// The header goes first, so the streams suspend before their slots go. The
// metadata goes too, so a sign after the wipe starts from free slots, as it
// does on a disk never signed.
func Wipe(path string, maxSlots int) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	f, err := openSync(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := writeHeaderBlock(f, path, directio.AlignedBlock(PageSize)); err != nil {
		return fmt.Errorf("write empty block: %w", err)
	}
	if err := zeroPages(f, 1, maxSlots, deviceSize(f)); err != nil {
		return fmt.Errorf("zero the metadata area: %w", err)
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
