package resfshost

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The name wipefs gives a backup says the offset of the signature, which the
// dd restoring it seeks to.
func TestWipeBackupNameSaysTheOffset(t *testing.T) {
	m := wipeBackupName.FindStringSubmatch("wipefs-loop1-0x00000438.bak")
	require.NotNil(t, m)
	assert.Equal(t, "00000438", m[1])
	m = wipeBackupName.FindStringSubmatch("wipefs-mapper-data-v-0x00010040.bak")
	require.NotNil(t, m)
	assert.Equal(t, "00010040", m[1])
	assert.Nil(t, wipeBackupName.FindStringSubmatch("notes.txt"))
}

// A btrfs subvolume is selected by its path or by its id: either shares its
// device with the other subvolumes.
func TestMountsSubvolume(t *testing.T) {
	assert.True(t, mountsSubvolume("rw,subvol=/data"))
	assert.True(t, mountsSubvolume("subvolid=257,noatime"))
	assert.False(t, mountsSubvolume("rw,noatime"))
	assert.False(t, mountsSubvolume(""))
}

// The offsets of "wipefs --parsable", with or without its header.
func TestParsableOffsets(t *testing.T) {
	assert.Equal(t, []int64{0x438}, parsableOffsets("0x438,82c9d755-6c1e,,ext4\n"))
	assert.Equal(t, []int64{0x0, 0x10040}, parsableOffsets("# offset,uuid,label,type\n0x0,a,,xfs\n0x10040,b,,btrfs\n"))
	assert.Empty(t, parsableOffsets(""))
}
