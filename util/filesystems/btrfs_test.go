package filesystems

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBTRFSParseShowDevices(t *testing.T) {
	out := `Label: 'dev.btrfs'  uuid: 0d05d0b9-ffab-4ab8-b923-15a38ec806d5
	Total devices 2 FS bytes used 48.92MiB
	devid    1 size 5.00GiB used 1.53GiB path /dev/vdb
	devid    2 size 5.00GiB used 1.51GiB path /dev/mapper/with space

`
	assert.Equal(t, []btrfsDevice{
		{id: "1", path: "/dev/vdb"},
		{id: "2", path: "/dev/mapper/with space"},
	}, parseShowDevices([]byte(out)))
}

func TestBTRFSSubvolOf(t *testing.T) {
	assert.Equal(t, "data", subvolOf("rw,subvol=data,noatime"))
	assert.Equal(t, "a/b", subvolOf("subvol=/a/b/"))
	assert.Equal(t, "", subvolOf("rw,subvolid=257"))
	assert.Equal(t, "", subvolOf(""))
}

func TestBTRFSIsRegistered(t *testing.T) {
	fs := FromType("btrfs")
	_, isFormateder := fs.(IsFormateder)
	_, isMKFSer := fs.(MKFSer)
	_, isGrower := fs.(Grower)
	_, isProvisioner := fs.(MountOptionsProvisioner)
	assert.True(t, isFormateder && isMKFSer && isGrower && isProvisioner)
	assert.True(t, fs.IsMultiDevice())
}
