package filesystems

type (
	T_Ext2 struct{ T }
)

func init() {
	registerFS(NewExt2())
}

func NewExt2() *T_Ext2 {
	t := T_Ext2{
		T{fsType: "ext2"},
	}
	return &t
}

func (t T_Ext2) CanFSCK() error {
	return extCanFSCK()
}

func (t T_Ext2) FSCK(s string) error {
	return extFSCK(s)
}

func (t T_Ext2) IsFormated(s string) (bool, error) {
	return extIsFormated(s)
}

func (t T_Ext2) MKFS(s string, args []string) error {
	return xMKFS("mkfs.ext2", s, args, t.log)
}
