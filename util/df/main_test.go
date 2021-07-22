package df

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsage(t *testing.T) {
	entries, err := Usage()
	require.Nil(t, err)
	require.Greater(t, len(entries), 0)
	nbWithFree := 0
	for _, e := range entries {
		if e.Free > 0 {
			nbWithFree = nbWithFree + 1
		}
	}
	assert.Greaterf(t, nbWithFree, 0, "no entry with free space: %s", entries)
}

func TestInode(t *testing.T) {
	entries, err := Inode()
	require.Nil(t, err)
	require.Greater(t, len(entries), 0)
	nbWithFree := 0
	for _, e := range entries {
		if e.Free > 0 {
			nbWithFree = nbWithFree + 1
		}
	}
	assert.Greaterf(t, nbWithFree, 0, "no entry with free space: %s", entries)
}

func TestMountUsage(t *testing.T) {
	entries, err := MountUsage("/")
	require.Nil(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "/", entries[0].MountPoint)
}

func TestTypeMountUsage(t *testing.T) {
	t.Run("must succeed on one fs type", func(t *testing.T) {
		succeed := 0
		for _, fsType := range []string{"xfs", "ext3", "ext4", "zfs", "ufs", "apfs"} {
			if _, err := TypeMountUsage(fsType, "/"); err == nil {
				succeed = succeed + 1
			}
		}
		assert.Equal(t, 1, succeed)
	})

	t.Run("return error if no such fs type", func(t *testing.T) {
		entries, err := TypeMountUsage("suchDoesNotExists", "/")
		require.NotNil(t, err, entries)
	})
}
