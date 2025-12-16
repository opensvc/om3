package df

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsage(t *testing.T) {
	ctx := t.Context()
	entries, err := Usage(ctx)
	if exitError, ok := err.(*exec.ExitError); ok {
		t.Logf("got error with stderr: %v", string(exitError.Stderr))
	}
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
	ctx := t.Context()
	entries, err := Inode(ctx)
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
	ctx := t.Context()
	entries, err := MountUsage(ctx, "/")
	require.Nil(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "/", entries[0].MountPoint)
}

func TestTypeMountUsage(t *testing.T) {
	ctx := t.Context()
	t.Run("must succeed on one fs type", func(t *testing.T) {
		succeed := 0
		for _, fsType := range []string{"xfs", "ext3", "ext4", "zfs", "ufs", "apfs"} {
			if _, err := TypeMountUsage(ctx, fsType, "/"); err == nil {
				succeed = succeed + 1
			}
		}
		assert.Equal(t, 1, succeed)
	})

	t.Run("return error if no such fs type", func(t *testing.T) {
		entries, err := TypeMountUsage(ctx, "suchDoesNotExists", "/")
		require.NotNil(t, err, entries)
	})
}
