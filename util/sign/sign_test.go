package sign

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSign(t *testing.T) {
	tempDir := t.TempDir()
	diskPath := filepath.Join(tempDir, "disk.img")

	f, err := os.Create(diskPath)
	require.NoError(t, err)
	err = f.Truncate(PageSizeInt64 * 10)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	hasSig, err := EnsureSignature(diskPath)
	require.NoError(t, err)
	require.False(t, hasSig)

	err = Sign(diskPath, SignOptions{Force: true})
	require.NoError(t, err)

	hasSig, err = EnsureSignature(diskPath)
	require.NoError(t, err)
	require.True(t, hasSig)

	sig, err := getSignature(diskPath)
	require.NoError(t, err)
	require.Equal(t, []byte(HBDiskSignature), sig)

	err = Wipe(diskPath, 0)
	require.NoError(t, err)

	hasSig, err = EnsureSignature(diskPath)
	require.NoError(t, err)
	require.False(t, hasSig)
}

func TestVerifyHeader(t *testing.T) {
	tempDir := t.TempDir()
	diskPath := filepath.Join(tempDir, "disk.img")

	f, err := os.Create(diskPath)
	require.NoError(t, err)
	err = f.Truncate(PageSizeInt64)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	err = Sign(diskPath, SignOptions{Force: true})
	require.NoError(t, err)

	block, err := os.ReadFile(diskPath)
	require.NoError(t, err)
	require.NoError(t, VerifyHeader(block))

	corruptedChecksum := make([]byte, len(block))
	copy(corruptedChecksum, block)
	corruptedChecksum[HBCrcOffset] ^= 0xff
	require.ErrorIs(t, VerifyHeader(corruptedChecksum), ErrChecksumMismatch)

	corruptedMagic := make([]byte, len(block))
	copy(corruptedMagic, block)
	corruptedMagic[HBMagicOffset] ^= 0xff
	require.ErrorIs(t, VerifyHeader(corruptedMagic), ErrWrongSignature)

	require.EqualError(t, VerifyHeader(block[:HBHeaderSize-1]), fmt.Sprintf("%s: %d", ErrBlockTooSmall, HBHeaderSize-1))

	// Legacy signature at offset 0
	legacyBlock := make([]byte, len(block))
	copy(legacyBlock[:len(HBDiskSignature)], HBDiskSignature)
	require.ErrorIs(t, VerifyHeader(legacyBlock), ErrLegacySignature)
}

func TestLegacySignatureDisk(t *testing.T) {
	tempDir := t.TempDir()
	diskPath := filepath.Join(tempDir, "legacy_disk.img")

	legacyBlock := make([]byte, PageSize)
	copy(legacyBlock[:len(HBDiskSignature)], HBDiskSignature)
	err := os.WriteFile(diskPath, legacyBlock, 0644)
	require.NoError(t, err)

	hasSig, err := EnsureSignature(diskPath)
	require.NoError(t, err)
	require.True(t, hasSig)

	sig, err := getSignature(diskPath)
	require.NoError(t, err)
	require.Equal(t, []byte(HBDiskSignature), sig)
}

// A filesystem refusing direct i/o, as tmpfs, is signed and wiped with
// synchronous i/o: a device never refuses it, and the files the tests sign
// may live on one.
func TestSignOnAFilesystemRefusingDirectIO(t *testing.T) {
	dir, err := os.MkdirTemp("/dev/shm", "sign-test-")
	if err != nil {
		t.Skip("no tmpfs at /dev/shm")
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	diskPath := filepath.Join(dir, "disk.img")
	f, err := os.Create(diskPath)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(PageSizeInt64*2))
	require.NoError(t, f.Close())

	require.NoError(t, Sign(diskPath, SignOptions{Force: true}))
	hasSig, err := EnsureSignature(diskPath)
	require.NoError(t, err)
	require.True(t, hasSig)

	require.NoError(t, Wipe(diskPath, 0))
	hasSig, err = EnsureSignature(diskPath)
	require.NoError(t, err)
	require.False(t, hasSig)
}

// newDisk returns a file of size bytes standing for a disk, its pages from
// the second one up to the page pages filled with what a filesystem left.
func newDisk(t *testing.T, size int64, garbagePages int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "disk.img")
	f, err := os.Create(p)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(size))
	for page := 1; page < garbagePages; page++ {
		_, err := f.WriteAt([]byte{0x04, 0xff, 0x07}, int64(page)*PageSizeInt64)
		require.NoError(t, err)
	}
	require.NoError(t, f.Close())
	return p
}

func pageFirstByte(t *testing.T, p string, page int) byte {
	t.Helper()
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return b[int64(page)*PageSizeInt64]
}

func withProbe(t *testing.T, kind string) {
	t.Helper()
	saved := probeSignature
	probeSignature = func(string) (string, error) { return kind, nil }
	t.Cleanup(func() { probeSignature = saved })
}

// A disk never signed gets its metadata area zeroed by the sign: the nodes
// took a page a filesystem left as a slot held, and claimed slots past the
// end of the disk.
func TestSignZeroesTheMetadataOfADiskNeverSigned(t *testing.T) {
	withProbe(t, "")
	p := newDisk(t, 64*PageSizeInt64, 40)

	require.NoError(t, Sign(p, SignOptions{MaxSlots: 32}))
	for page := 1; page < 32; page++ {
		require.Zero(t, pageFirstByte(t, p, page), "page %d", page)
	}
	require.NotZero(t, pageFirstByte(t, p, 35), "past the metadata area, untouched")
	hasSig, err := EnsureSignature(p)
	require.NoError(t, err)
	require.True(t, hasSig)
}

// A disk signed keeps its metadata, the slots of the nodes and of the other
// clusters sharing it, when signed again, and so does one whose header is of
// the earlier format, or damaged.
func TestSignKeepsTheMetadataOfAHeartbeatDisk(t *testing.T) {
	withProbe(t, "")
	for name, damage := range map[string]func(p string){
		"signed":  func(string) {},
		"damaged": func(p string) { corruptByte(t, p, HBCrcOffset) },
		"legacy": func(p string) {
			f, err := os.OpenFile(p, os.O_RDWR, 0)
			require.NoError(t, err)
			block := make([]byte, PageSize)
			copy(block, HBDiskSignature)
			_, err = f.WriteAt(block, 0)
			require.NoError(t, err)
			require.NoError(t, f.Close())
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := newDisk(t, 64*PageSizeInt64, 0)
			require.NoError(t, Sign(p, SignOptions{MaxSlots: 32}))
			f, err := os.OpenFile(p, os.O_RDWR, 0)
			require.NoError(t, err)
			_, err = f.WriteAt([]byte("node1\x00"), 3*PageSizeInt64)
			require.NoError(t, err)
			require.NoError(t, f.Close())
			damage(p)

			require.NoError(t, Sign(p, SignOptions{MaxSlots: 32}))
			require.Equal(t, byte('n'), pageFirstByte(t, p, 3), "the slot of node1 is kept")
		})
	}
}

func corruptByte(t *testing.T, p string, offset int64) {
	t.Helper()
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	require.NoError(t, err)
	_, err = f.WriteAt([]byte{0xaa}, offset)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

// A disk holding a signature not of a heartbeat, as a filesystem, is not
// signed unless forced: the sign erases its first pages.
func TestSignRefusesADiskHoldingAFilesystem(t *testing.T) {
	withProbe(t, "TYPE=ext4")
	p := newDisk(t, 64*PageSizeInt64, 40)

	err := Sign(p, SignOptions{MaxSlots: 32})
	require.ErrorIs(t, err, ErrForeignSignature)
	require.ErrorContains(t, err, "TYPE=ext4")
	hasSig, err := EnsureSignature(p)
	require.NoError(t, err)
	require.False(t, hasSig, "nothing written")
	require.NotZero(t, pageFirstByte(t, p, 5))

	require.NoError(t, Sign(p, SignOptions{MaxSlots: 32, Force: true}))
	require.Zero(t, pageFirstByte(t, p, 5))
}

// blkid tells a filesystem on a disk, when installed.
func TestBlkidProbeTellsAFilesystem(t *testing.T) {
	mkfs, err := exec.LookPath("mkfs.ext4")
	if err != nil {
		t.Skip("no mkfs.ext4")
	}
	if _, err := exec.LookPath("blkid"); err != nil {
		t.Skip("no blkid")
	}
	p := newDisk(t, 8*1024*1024, 0)
	require.NoError(t, exec.Command(mkfs, "-q", "-F", p).Run())
	kind, err := blkidProbe(p)
	require.NoError(t, err)
	require.Equal(t, "TYPE=ext4", kind)

	kind, err = blkidProbe(newDisk(t, 8*1024*1024, 0))
	require.NoError(t, err)
	require.Empty(t, kind)
}

// A disk too small for the metadata area and a data slot per node is not
// signed.
func TestSignRefusesADiskTooSmall(t *testing.T) {
	withProbe(t, "")
	p := newDisk(t, MinSize(32, 2)-1, 0)
	err := Sign(p, SignOptions{MaxSlots: 32, MinSize: MinSize(32, 2)})
	require.ErrorIs(t, err, ErrTooSmall)
	hasSig, err := EnsureSignature(p)
	require.NoError(t, err)
	require.False(t, hasSig)

	p = newDisk(t, MinSize(32, 2), 0)
	require.NoError(t, Sign(p, SignOptions{MaxSlots: 32, MinSize: MinSize(32, 2)}))
}

// A wipe erases the metadata area with the header, so a sign after it starts
// from free slots.
func TestWipeErasesTheMetadata(t *testing.T) {
	withProbe(t, "")
	p := newDisk(t, 64*PageSizeInt64, 0)
	require.NoError(t, Sign(p, SignOptions{MaxSlots: 32}))
	corruptByte(t, p, 3*PageSizeInt64)

	require.NoError(t, Wipe(p, 32))
	require.Zero(t, pageFirstByte(t, p, 0))
	require.Zero(t, pageFirstByte(t, p, 3))
}

func TestMinSize(t *testing.T) {
	require.Equal(t, int64(4*1024*1024+3*1024*1024), MinSize(1024, 2), "4 MiB of metadata, slot 0 unused, 2 nodes")
}

// A disk whose signatures no tool can read, blkid not installed, is not
// signed unless forced: it was signed as holding none, its first pages erased
// unseen.
func TestSignRefusesADiskNotProbed(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p := newDisk(t, 64*PageSizeInt64, 40)
	err := Sign(p, SignOptions{MaxSlots: 32})
	require.ErrorIs(t, err, ErrNotProbed)
	require.ErrorContains(t, err, "blkid is not installed")
	require.NotZero(t, pageFirstByte(t, p, 5), "nothing erased")

	require.NoError(t, Sign(p, SignOptions{MaxSlots: 32, Force: true}))
	require.Zero(t, pageFirstByte(t, p, 5))
}
