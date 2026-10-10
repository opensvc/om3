package resfshost

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBtrfsSubvolKeywordReachesTheMountOptions(t *testing.T) {
	r := &T{Type: "btrfs", MountOptions: "noatime,subvol=/data/,compress=zstd", Subvol: "data"}
	assert.Empty(t, r.btrfsConflicts(), "the same subvolume, written another way")
	assert.Equal(t, "noatime,compress=zstd,subvol=data", r.mountOptions(), "the keyword sets subvol=")

	r = &T{Type: "btrfs", MountOptions: "noatime", Subvol: "/a/b"}
	assert.Equal(t, "noatime,subvol=a/b", r.mountOptions())

	r = &T{Type: "btrfs", MountOptions: "noatime,subvol=data"}
	assert.Equal(t, "noatime,subvol=data", r.mountOptions(), "without the keyword, mnt_opt is the mount options")

	r = &T{Type: "ext4", MountOptions: "noatime", Subvol: "data"}
	assert.Equal(t, "noatime", r.mountOptions(), "only a btrfs has subvolumes")
	assert.Empty(t, r.btrfsConflicts())
}

func TestBtrfsConflictsAreRefused(t *testing.T) {
	for name, r := range map[string]*T{
		"another subvol": {Type: "btrfs", MountOptions: "subvol=foo", Subvol: "data"},
		"a subvolid":     {Type: "btrfs", MountOptions: "subvolid=257", Subvol: "data"},
		"another label":  {Type: "btrfs", MKFSOptions: []string{"-L", "other"}, BtrfsLabel: "fs1"},
		"--label=":       {Type: "btrfs", MKFSOptions: []string{"--label=other"}, BtrfsLabel: "fs1"},
	} {
		t.Run(name, func(t *testing.T) {
			require.Len(t, r.btrfsConflicts(), 1)
			assert.Contains(t, r.btrfsConflicts()[0].Error(), "remove one")
		})
	}
}

func TestBtrfsLabelKeywordReachesTheMkfsOptions(t *testing.T) {
	r := &T{Type: "btrfs", MKFSOptions: []string{"-m", "dup"}, BtrfsLabel: "fs1"}
	assert.Equal(t, []string{"-m", "dup", "-L", "fs1"}, r.mkfsOptions())
	assert.Equal(t, []string{"-m", "dup"}, r.MKFSOptions, "the configuration is not changed")

	r = &T{Type: "btrfs", MKFSOptions: []string{"-L", "fs1"}, BtrfsLabel: "fs1"}
	assert.Equal(t, []string{"-L", "fs1"}, r.mkfsOptions(), "a label set the same way twice is set once")
	assert.Empty(t, r.btrfsConflicts())

	r = &T{Type: "ext4", MKFSOptions: []string{"-m", "1"}, BtrfsLabel: "fs1"}
	assert.Equal(t, []string{"-m", "1"}, r.mkfsOptions())
}

func TestMkfsLabel(t *testing.T) {
	assert.Equal(t, "a", mkfsLabel([]string{"-f", "-L", "a"}))
	assert.Equal(t, "b", mkfsLabel([]string{"--label", "b"}))
	assert.Equal(t, "c", mkfsLabel([]string{"--label=c"}))
	assert.Equal(t, "d", mkfsLabel([]string{"-Ld"}))
	assert.Equal(t, "", mkfsLabel([]string{"-m", "dup"}))
	assert.Equal(t, "", mkfsLabel([]string{"-L"}))
}
