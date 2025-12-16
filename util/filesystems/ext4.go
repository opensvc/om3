package filesystems

import "context"

type (
	Ext4 struct{ T }
)

func init() {
	registerFS(NewExt4())
}

func NewExt4() *Ext4 {
	t := Ext4{
		T{fsType: "ext4"},
	}
	return &t
}

func (t Ext4) CanFSCK() error {
	return extCanFSCK()
}

func (t Ext4) FSCK(ctx context.Context, s string) error {
	return extFSCK(ctx, s)
}

func (t Ext4) IsFormated(ctx context.Context, s string) (bool, error) {
	return extIsFormated(ctx, s)
}

func (t Ext4) MKFS(ctx context.Context, s string, args []string) error {
	return xMKFS(ctx, "mkfs.ext4", s, args, t.log)
}
