package filesystems

import "testing"

var (
	_ IsFormateder = (*XFS)(nil)
	_ MKFSer       = (*XFS)(nil)

	_ IsFormateder = (*Ext2)(nil)
	_ MKFSer       = (*Ext2)(nil)

	_ IsFormateder = (*Ext3)(nil)
	_ MKFSer       = (*Ext3)(nil)

	_ IsFormateder = (*Ext4)(nil)
	_ MKFSer       = (*Ext4)(nil)

	_ IsFormateder = (*Ext2)(nil)
	_ MKFSer       = (*Ext2)(nil)

	_ IsFormateder = (*Ext2)(nil)
	_ MKFSer       = (*Ext2)(nil)
)

func TestFS(t *testing.T) {
	// Ensure interfaces are implemented
}
