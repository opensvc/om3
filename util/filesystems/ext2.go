package filesystems

import "context"

type (
	Ext2 struct{ T }
)

func init() {
	registerFS(NewExt2())
}

func NewExt2() *Ext2 {
	t := Ext2{
		T{fsType: "ext2"},
	}
	return &t
}

func (t Ext2) CanFSCK() error {
	return extCanFSCK()
}

func (t Ext2) FSCK(ctx context.Context, s string) error {
	return extFSCK(ctx, s)
}

func (t Ext2) IsFormated(ctx context.Context, s string) (bool, error) {
	return extIsFormated(ctx, s)
}

func (t Ext2) MKFS(ctx context.Context, s string, args []string) error {
	return xMKFS(ctx, "mkfs.ext2", s, args, t.log)
}
