package filesystems

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

func (t Ext2) FSCK(s string) error {
	return extFSCK(s)
}

func (t Ext2) IsFormated(s string) (bool, error) {
	return extIsFormated(s)
}

func (t Ext2) MKFS(s string, args []string) error {
	return xMKFS("mkfs.ext2", s, args, t.log)
}
