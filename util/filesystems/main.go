package filesystem

import (
	"errors"
	"fmt"

	"opensvc.com/opensvc/util/device"
)

type (
	T struct {
		name          string
		isNetworked   bool
		isMultiDevice bool
		isFileBacked  bool
		isVirtual     bool
		fsck          func(s string) error
		canFSCK       func() error
		isFormated    func(s string) (bool, error)
		mkfs          func(s string) error
	}

	deviceLister interface {
		Devices() ([]device.T, error)
	}
)

var (
	T_Shm       T = T{name: "shm", isVirtual: true}
	T_ShmFS     T = T{name: "shmfs", isVirtual: true}
	T_TmpFS     T = T{name: "tmpfs", isVirtual: true}
	T_None      T = T{name: "none", isVirtual: true}
	T_Bind      T = T{name: "bind", isFileBacked: true}
	T_LoFS      T = T{name: "lofs", isFileBacked: true}
	T_Ext       T = T{name: "ext"}
	T_AdvFS     T = T{name: "btrfs", isMultiDevice: true}
	T_BtrFS     T = T{name: "btrfs", isMultiDevice: true}
	T_ZFS       T = T{name: "zfs", isMultiDevice: true}
	T_VFAT      T = T{name: "vfat"}
	T_ReiserFS  T = T{name: "reiserfs"}
	T_JFS       T = T{name: "jfs"}
	T_JFS2      T = T{name: "jfs2"}
	T_BFS       T = T{name: "bfs"}
	T_MSDOS     T = T{name: "msdos"}
	T_UFS       T = T{name: "ufs"}
	T_UFS2      T = T{name: "ufs2"}
	T_Minix     T = T{name: "minix"}
	T_XIA       T = T{name: "xia"}
	T_UMSDOS    T = T{name: "umsdos"}
	T_HPFS      T = T{name: "hpfs"}
	T_NTFS      T = T{name: "ntfs"}
	T_ReiserFS4 T = T{name: "reiserfs4"}
	T_VXFS      T = T{name: "vxfs"}
	T_HFS       T = T{name: "hfs"}
	T_HFSPlus   T = T{name: "hfsplus"}
	T_QNX4      T = T{name: "qnx4"}
	T_OCFS      T = T{name: "ocfs"}
	T_OCFS2     T = T{name: "ocfs2"}
	T_NilFS     T = T{name: "nilfs"}
	T_JFFS      T = T{name: "jffs"}
	T_JFFS2     T = T{name: "jffs2"}
	T_Tux3      T = T{name: "tux3"}
	T_F2FS      T = T{name: "f2fs"}
	T_LogFS     T = T{name: "logfs"}
	T_GFS       T = T{name: "gfs"}
	T_GFS2      T = T{name: "gfs2"}
	T_NFS       T = T{name: "nfs", isNetworked: true}
	T_NFS4      T = T{name: "nfs4", isNetworked: true}
	T_SmbFS     T = T{name: "smbfs", isNetworked: true}
	T_CIFS      T = T{name: "cifs", isNetworked: true}
	T_9PFS      T = T{name: "9pfs", isNetworked: true}
	T_GPFS      T = T{name: "gpfs", isNetworked: true}
	T_AFS       T = T{name: "afs", isNetworked: true}
	T_NCPFS     T = T{name: "ncpfs", isNetworked: true}
	T_GlusterFS T = T{name: "glusterfs", isNetworked: true}
	T_CephFS    T = T{name: "cephfs", isNetworked: true}

	fromString = map[string]T{
		"shm":       T_Shm,
		"shmfs":     T_ShmFS,
		"tmpfs":     T_TmpFS,
		"none":      T_None,
		"bind":      T_Bind,
		"lofs":      T_LoFS,
		"ext":       T_Ext,
		"ext2":      T_Ext2,
		"ext3":      T_Ext3,
		"ext4":      T_Ext4,
		"xfs":       T_XFS,
		"btrfs":     T_BtrFS,
		"advfs":     T_AdvFS,
		"zfs":       T_ZFS,
		"vfat":      T_VFAT,
		"reiserfs":  T_ReiserFS,
		"jfs":       T_JFS,
		"jfs2":      T_JFS2,
		"bfs":       T_BFS,
		"msdos":     T_MSDOS,
		"ufs":       T_UFS,
		"ufs2":      T_UFS2,
		"minix":     T_Minix,
		"xia":       T_XIA,
		"umsdos":    T_UMSDOS,
		"hpfs":      T_HPFS,
		"ntfs":      T_NTFS,
		"reiserfs4": T_ReiserFS4,
		"vxfs":      T_VXFS,
		"hfs":       T_HFS,
		"hfsplus":   T_HFSPlus,
		"qnx4":      T_QNX4,
		"ocfs":      T_OCFS,
		"ocfs2":     T_OCFS2,
		"nilfs":     T_NilFS,
		"jffs":      T_JFFS,
		"jffs2":     T_JFFS2,
		"tux3":      T_Tux3,
		"f2fs":      T_F2FS,
		"logfs":     T_LogFS,
		"gfs":       T_GFS,
		"gfs2":      T_GFS2,
		"nfs":       T_NFS,
		"nfs4":      T_NFS4,
		"smbfs":     T_SmbFS,
		"cifs":      T_CIFS,
		"9pfs":      T_9PFS,
		"gpfs":      T_GPFS,
		"afs":       T_AFS,
		"ncpfs":     T_NCPFS,
		"glusterfs": T_GlusterFS,
		"cephfs":    T_CephFS,
	}

	Converter T
)

func (t T) String() string {
	return t.name
}

func (t T) IsZero() bool {
	return t.name == ""
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

func (t T) CanFSCK() error {
	if t.canFSCK == nil {
		return nil
	}
	return t.canFSCK()
}

func (t T) HasFSCK() bool {
	return t.fsck != nil
}

func (t T) FSCK(dl deviceLister) error {
	if !t.HasFSCK() {
		return nil
	}
	devices, err := dl.Devices()
	if err != nil {
		return err
	}
	for _, dev := range devices {
		if err := t.fsck(dev.String()); err != nil {
			return err
		}
	}
	return nil
}

func (t T) IsFormated(dl deviceLister) (bool, error) {
	if t.isFormated == nil {
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
		v, err := t.isFormated(dev.String())
		if err != nil {
			return false, err
		}
		if !v {
			return false, nil
		}
	}
	return true, nil
}

func FromType(s string) T {
	if t, ok := fromString[s]; ok {
		return t
	}
	return T{}
}

func (t T) Convert(s string) (interface{}, error) {
	if t, ok := fromString[s]; ok {
		return t, nil
	}
	return T{}, fmt.Errorf("unknown filesystem: %s", s)
}
