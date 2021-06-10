package filesystems

import (
	"errors"

	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/xmap"
)

type (
	T struct {
		fsType        string
		isNetworked   bool
		isMultiDevice bool
		isFileBacked  bool
		isVirtual     bool
	}

	deviceLister interface {
		Devices() ([]device.T, error)
	}

	I interface {
		String() string
		Type() string
		IsZero() bool
		IsNetworked() bool
		IsVirtual() bool
		IsFileBacked() bool
		IsMultiDevice() bool
	}
	FSCKer interface {
		FSCK(string) error
	}
	CanFSCKer interface {
		CanFSCK() error
	}
	IsFormateder interface {
		IsFormated(string) (bool, error)
	}
)

var (
	db = make(map[string]interface{})
)

func init() {
	registerFS(T{fsType: "shm", isVirtual: true})
	registerFS(T{fsType: "shmfs", isVirtual: true})
	registerFS(T{fsType: "tmpfs", isVirtual: true})
	registerFS(T{fsType: "none", isVirtual: true})
	registerFS(T{fsType: "bind", isFileBacked: true})
	registerFS(T{fsType: "lofs", isFileBacked: true})
	registerFS(T{fsType: "btrfs", isMultiDevice: true})
	registerFS(T{fsType: "zfs", isMultiDevice: true})
	registerFS(T{fsType: "vfat"})
	registerFS(T{fsType: "reiserfs"})
	registerFS(T{fsType: "jfs"})
	registerFS(T{fsType: "jfs2"})
	registerFS(T{fsType: "bfs"})
	registerFS(T{fsType: "msdos"})
	registerFS(T{fsType: "ufs"})
	registerFS(T{fsType: "ufs2"})
	registerFS(T{fsType: "minix"})
	registerFS(T{fsType: "xia"})
	registerFS(T{fsType: "umsdos"})
	registerFS(T{fsType: "hpfs"})
	registerFS(T{fsType: "ntfs"})
	registerFS(T{fsType: "reiserfs4"})
	registerFS(T{fsType: "vxfs"})
	registerFS(T{fsType: "hfs"})
	registerFS(T{fsType: "hfsplus"})
	registerFS(T{fsType: "qnx4"})
	registerFS(T{fsType: "ocfs"})
	registerFS(T{fsType: "ocfs2"})
	registerFS(T{fsType: "nilfs"})
	registerFS(T{fsType: "jffs"})
	registerFS(T{fsType: "jffs2"})
	registerFS(T{fsType: "tux3"})
	registerFS(T{fsType: "f2fs"})
	registerFS(T{fsType: "logfs"})
	registerFS(T{fsType: "gfs"})
	registerFS(T{fsType: "gfs2"})
	registerFS(T{fsType: "nfs", isNetworked: true})
	registerFS(T{fsType: "nfs4", isNetworked: true})
	registerFS(T{fsType: "smbfs", isNetworked: true})
	registerFS(T{fsType: "cifs", isNetworked: true})
	registerFS(T{fsType: "9pfs", isNetworked: true})
	registerFS(T{fsType: "gpfs", isNetworked: true})
	registerFS(T{fsType: "afs", isNetworked: true})
	registerFS(T{fsType: "ncpfs", isNetworked: true})
	registerFS(T{fsType: "glusterfs", isNetworked: true})
	registerFS(T{fsType: "cephfs", isNetworked: true})
}

func registerFS(fs I) {
	db[fs.Type()] = fs
}

func (t T) String() string {
	return t.fsType
}

func (t T) Type() string {
	return t.fsType
}

func (t T) IsZero() bool {
	return t.fsType == ""
}

func (t T) IsNetworked() bool {
	return t.isNetworked
}

func (t T) IsVirtual() bool {
	return t.isVirtual
}

func (t T) IsFileBacked() bool {
	return t.isFileBacked
}

func (t T) IsMultiDevice() bool {
	return t.isMultiDevice
}

func CanFSCK(fs interface{}) error {
	if i, ok := fs.(CanFSCKer); !ok {
		return nil
	} else {
		return i.CanFSCK()
	}
}

func HasFSCK(fs interface{}) bool {
	_, ok := fs.(FSCKer)
	return ok
}

func DevicesFSCK(fs interface{}, dl deviceLister) error {
	i, ok := fs.(FSCKer)
	if !ok {
		return nil
	}
	devices, err := dl.Devices()
	if err != nil {
		return err
	}
	for _, dev := range devices {
		if err := i.FSCK(dev.String()); err != nil {
			return err
		}
	}
	return nil
}

func DevicesFormated(fs interface{}, dl deviceLister) (bool, error) {
	i, ok := fs.(IsFormateder)
	if !ok {
		return false, errors.New("isFormated is not implemented")
	}
	devices, err := dl.Devices()
	if err != nil {
		return false, err
	}
	if len(devices) == 0 {
		return false, errors.New("no devices")
	}
	for _, dev := range devices {
		v, err := i.IsFormated(dev.String())
		if err != nil {
			return false, err
		}
		if !v {
			return false, nil
		}
	}
	return true, nil
}

func FromType(s string) I {
	if t, ok := db[s]; ok {
		return t.(I)
	}
	return T{}
}

func Types() []string {
	return xmap.Keys(db)
}
