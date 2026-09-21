package sign

import (
	"fmt"
	"os"
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

	err = CreateAndFillDisk(diskPath)
	require.NoError(t, err)

	hasSig, err = EnsureSignature(diskPath)
	require.NoError(t, err)
	require.True(t, hasSig)

	sig, err := getSignature(diskPath)
	require.NoError(t, err)
	require.Equal(t, []byte(HBDiskSignature), sig)

	err = RemoveHeaderFromDisk(diskPath)
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

	err = CreateAndFillDisk(diskPath)
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
