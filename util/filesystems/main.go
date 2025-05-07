package filesystems

import (
	"bufio"
	"errors"
	"os"
	"strings"

	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/xmap"
)

type (
	T struct {
		fsType        string
		isNetworked   bool
		isMultiDevice bool
		isFileBacked  bool
		isVirtual     bool
		log           *plog.Logger
	}

	subDeviceLister interface {
		SubDevices() device.L
	}

	I interface {
		String() string
		Type() string
		IsZero() bool
		IsNetworked() bool
		IsVirtual() bool
		IsFileBacked() bool
		IsMultiDevice() bool
		Mount(string, string, string) error
		Umount(string) error
		KillUsers(string) error
		Log() *plog.Logger
		SetLog(*plog.Logger)
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
	IsCapabler interface {
		IsCapable() bool
	}
	MKFSer interface {
		MKFS(string, []string) error
	}
)

var (
	availTypesCache availTypesM
	db              = make(map[string]any)
)

func init() {
	registerFS(&T{fsType: "tmpfs", isVirtual: true})
	registerFS(&T{fsType: "none", isFileBacked: true})
	registerFS(&T{fsType: "bind", isFileBacked: true})
	registerFS(&T{fsType: "lofs", isFileBacked: true})
	registerFS(&T{fsType: "btrfs", isMultiDevice: true})
	registerFS(&T{fsType: "vfat"})
	registerFS(&T{fsType: "reiserfs"})
	registerFS(&T{fsType: "jfs"})
	registerFS(&T{fsType: "jfs2"})
	registerFS(&T{fsType: "bfs"})
	registerFS(&T{fsType: "msdos"})
	registerFS(&T{fsType: "ufs"})
	registerFS(&T{fsType: "ufs2"})
	registerFS(&T{fsType: "minix"})
	registerFS(&T{fsType: "xia"})
	registerFS(&T{fsType: "umsdos"})
	registerFS(&T{fsType: "hpfs"})
	registerFS(&T{fsType: "ntfs"})
	registerFS(&T{fsType: "reiserfs4"})
	registerFS(&T{fsType: "vxfs"})
	registerFS(&T{fsType: "hfs"})
	registerFS(&T{fsType: "hfsplus"})
	registerFS(&T{fsType: "qnx4"})
	registerFS(&T{fsType: "ocfs"})
	registerFS(&T{fsType: "ocfs2"})
	registerFS(&T{fsType: "nilfs"})
	registerFS(&T{fsType: "jffs"})
	registerFS(&T{fsType: "jffs2"})
	registerFS(&T{fsType: "tux3"})
	registerFS(&T{fsType: "f2fs"})
	registerFS(&T{fsType: "logfs"})
	registerFS(&T{fsType: "gfs"})
	registerFS(&T{fsType: "gfs2"})
	registerFS(&T{fsType: "nfs", isNetworked: true})
	registerFS(&T{fsType: "nfs4", isNetworked: true})
	registerFS(&T{fsType: "smbfs", isNetworked: true})
	registerFS(&T{fsType: "cifs", isNetworked: true})
	registerFS(&T{fsType: "9pfs", isNetworked: true})
	registerFS(&T{fsType: "gpfs", isNetworked: true})
	registerFS(&T{fsType: "afs", isNetworked: true})
	registerFS(&T{fsType: "ncpfs", isNetworked: true})
	registerFS(&T{fsType: "glusterfs", isNetworked: true})
	registerFS(&T{fsType: "cephfs", isNetworked: true})
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

func (t T) Log() *plog.Logger {
	return t.log
}

func (t *T) SetLog(log *plog.Logger) {
	t.log = log
}

func IsCapable(t string) bool {
	fs := FromType(t)
	if i, ok := fs.(IsCapabler); ok {
		if !i.IsCapable() {
			return false
		}
	}
	if !availTypes().Has(t) && !hasKMod(t) {
		return false
	}
	return true
}

func CanFSCK(fs any) error {
	if i, ok := fs.(CanFSCKer); !ok {
		return nil
	} else {
		return i.CanFSCK()
	}
}

func HasFSCK(fs any) bool {
	_, ok := fs.(FSCKer)
	return ok
}

func DevicesFSCK(fs any, dl subDeviceLister) error {
	i, ok := fs.(FSCKer)
	if !ok {
		return nil
	}
	devices := dl.SubDevices()
	for _, dev := range devices {
		if err := i.FSCK(dev.Path()); err != nil {
			return err
		}
	}
	return nil
}

func DevicesFormated(fs any, dl subDeviceLister) (bool, error) {
	i, ok := fs.(IsFormateder)
	if !ok {
		return false, errors.New("isFormated is not implemented")
	}
	devices := dl.SubDevices()
	if len(devices) == 0 {
		return false, errors.New("no devices")
	}
	for _, dev := range devices {
		v, err := i.IsFormated(dev.Path())
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
	return &T{}
}

func Types() []string {
	return xmap.Keys(db)
}

type availTypesM map[string]any

func (m availTypesM) Has(s string) bool {
	if m == nil {
		return false
	}
	_, ok := m[s]
	return ok
}

func hasKMod(s string) bool {
	cmd := command.New(
		command.WithName("modinfo"),
		command.WithVarArgs(s),
	)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func availTypes() availTypesM {
	if availTypesCache != nil {
		return availTypesCache
	}
	m := make(availTypesM)
	f, err := os.Open("/proc/filesystems")
	if err != nil {
		return m
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		fields := strings.Fields(s)
		n := len(fields)
		s = fields[n-1]
		if s != "" {
			m[s] = nil
		}
	}
	availTypesCache = m
	return m
}
