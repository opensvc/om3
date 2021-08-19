package filesystems

type (
	T_Ext4 struct{ T }
)

func init() {
	registerFS(NewExt4())
}

func NewExt4() *T_Ext4 {
	t := T_Ext4{
		T{fsType: "ext4"},
	}
	return &t
}

func (t T_Ext4) CanFSCK() error {
	return extCanFSCK()
}

func (t T_Ext4) FSCK(s string) error {
	return extFSCK(s)
}

func (t T_Ext4) IsFormated(s string) (bool, error) {
	return extIsFormated(s)
}

func (t T_Ext4) MKFS(s string, args []string) error {
	return xMKFS("mkfs.ext4", s, args, t.log)
}
