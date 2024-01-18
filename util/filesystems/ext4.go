package filesystems

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

func (t Ext4) FSCK(s string) error {
	return extFSCK(s)
}

func (t Ext4) IsFormated(s string) (bool, error) {
	return extIsFormated(s)
}

func (t Ext4) MKFS(s string, args []string) error {
	return xMKFS("mkfs.ext4", s, args, t.log)
}
