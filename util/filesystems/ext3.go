package filesystems

import "context"

type (
	Ext3 struct{ T }
)

func init() {
	registerFS(NewExt3())
}

func NewExt3() *Ext3 {
	t := Ext3{
		T{fsType: "ext3"},
	}
	return &t
}

func (t Ext3) CanFSCK() error {
	return extCanFSCK()
}

func (t Ext3) FSCK(ctx context.Context, s string) error {
	return extFSCK(ctx, s)
}

func (t Ext3) IsFormated(ctx context.Context, s string) (bool, error) {
	return extIsFormated(ctx, s)
}

func (t Ext3) MKFS(ctx context.Context, s string, args []string) error {
	return xMKFS(ctx, "mkfs.ext3", s, args, t.log)
}

func (t Ext3) Grow(ctx context.Context, dev, mountPoint string) error {
	return extGrow(ctx, t.Log(), dev, mountPoint)
}

func (t Ext3) CanShrink(ctx context.Context, dev, mountPoint string) error {
	return extCanShrink(ctx, dev, mountPoint)
}

func (t Ext3) Shrink(ctx context.Context, dev, mountPoint string, size int64) error {
	return extShrink(ctx, t.Log(), dev, mountPoint, size)
}
