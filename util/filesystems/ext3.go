package filesystems

type (
	T_Ext3 struct{ T }
)

func init() {
	registerFS(NewExt3())
}

func NewExt3() *T_Ext3 {
	t := T_Ext3{
		T{fsType: "ext3"},
	}
	return &t
}

func (t T_Ext3) CanFSCK() error {
	return extCanFSCK()
}

func (t T_Ext3) FSCK(s string) error {
	return extFSCK(t.String())
}

func (t T_Ext3) IsFormated(s string) (bool, error) {
	return extIsFormated(s)
}

func (t T_Ext3) MKFS(s string) error {
	return xMKFS("mkfs.ext3", s)
}
