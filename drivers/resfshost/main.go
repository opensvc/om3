package resfshost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/filesystems"
	"github.com/opensvc/om3/v3/util/findmnt"
)

type (
	T struct {
		resource.T
		resource.Restart
		resource.SCSIPersistentReservation
		datarecv.DataRecv
		Path            naming.Path
		MountPoint      string         `json:"mnt"`
		Device          string         `json:"dev"`
		Type            string         `json:"type"`
		MountOptions    string         `json:"mnt_opt"`
		StatTimeout     *time.Duration `json:"stat_timeout"`
		Zone            string         `json:"zone"`
		PRKey           string         `json:"prkey"`
		MKFSOptions     []string       `json:"mkfs_opt"`
		User            *user.User     `json:"user"`
		Group           *user.Group    `json:"group"`
		Perm            *os.FileMode   `json:"perm"`
		SCSIReservation bool           `json:"scsireserv"`
		NoPreemptAbort  bool           `json:"no_preempt_abort"`
		PromoteRW       bool           `json:"promote_rw"`
		CheckRead       bool           `json:"check_read"`

		// Size and Mode are the size and the permissions of a tmpfs, the
		// filesystems holding them rather than a device.
		Size *int64       `json:"size"`
		Mode *os.FileMode `json:"mode"`

		// Subvol and BtrfsLabel are the subvolume a btrfs mounts and the label
		// it is formatted with.
		Subvol     string `json:"subvol"`
		BtrfsLabel string `json:"label"`

		// zone caches the mount point in the root directory of the
		// container Zone names.
		zone *Zone
	}

	IsFormateder interface {
		IsFormated(context.Context, string) (bool, error)
	}
	MKFSer interface {
		MKFS(string, []string) error
	}
)

const (
	defaultPerm = 0755
)

func NewF(s string) func() resource.Driver {
	n := func() resource.Driver {
		t := &T{Type: s}
		return t
	}
	return n
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Configure installs a resource backpointer in the DataStoreInstall
func (t *T) Configure() error {
	t.DataRecv.SetReceiver(t)
	return nil
}

func (t *T) Start(ctx context.Context) error {
	if _, err := t.resolveMountPoint(); err != nil {
		return err
	}
	if err := t.mount(ctx); err != nil {
		return err
	}
	if err := t.DataRecv.Do(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if _, err := t.resolveMountPoint(); err != nil {
		return err
	}
	if v, err := t.isMounted(ctx); err != nil {
		return err
	} else if !v {
		t.Log().Infof("%s already umounted from %s", t.devpath(ctx), t.mountPoint())
		return nil
	}
	mnt := t.mountPoint()
	fs := t.fs()
	if err := t.umountFS(ctx); err != nil {
		// The users of a zoned filesystem are not killed: fuser
		// resolves the path, which a process of the container could
		// lead to a host path.
		if file.IsProtected(mnt) || t.Zone != "" {
			return err
		}
		if errors.Is(err, syscall.EBUSY) {
			for _ = range 4 {
				fs.KillUsers(ctx, mnt)
				if err := fs.Umount(ctx, mnt); err == nil {
					return nil
				}
			}
		}
		return err
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	for _, s := range t.overriddenMountOptions() {
		t.StatusLog().Warn("mnt_opt %s", s)
	}
	for _, err := range t.btrfsConflicts() {
		t.StatusLog().Error("%s", err)
	}
	if t.Device == "" {
		t.StatusLog().Info("dev is not defined")
		return status.NotApplicable
	}
	if t.MountPoint == "" {
		t.StatusLog().Info("mnt is not defined")
		return status.NotApplicable
	}
	if _, err := t.resolveMountPoint(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	if v, err := t.isMounted(ctx); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if !v {
		return status.Down
	}
	if t.canCheckWriteAccess() {
		if err := t.checkWriteAccess(); err != nil {
			t.StatusLog().Error("%s", err)
			return status.Warn
		}
	} else if t.canCheckReadAccess() {
		if err := t.checkReadAccess(); err != nil {
			t.StatusLog().Error("%s", err)
			return status.Warn
		}
	}
	t.DataRecv.Status()
	return status.Up
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(ctx context.Context) string {
	devpath := t.devpath(ctx)
	mountPoint := t.mountPoint()
	var buff strings.Builder
	if devpath != "none" {
		buff.WriteString(devpath)
	}
	if mountPoint != "" {
		buff.WriteString("@")
		buff.WriteString(mountPoint)
	}
	return buff.String()
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

// UnprovisionAsLeader removes the mount point, as UnprovisionAsFollower
// does: it is on each node.
func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	if _, err := t.resolveMountPoint(); err != nil {
		return err
	}
	return t.removeMountPoint()
}

func (t *T) UnprovisionAsFollower(ctx context.Context) error {
	if _, err := t.resolveMountPoint(); err != nil {
		return err
	}
	return t.removeMountPoint()
}

// removeMountPoint removes the mount point a start of the resource created,
// a directory or the empty file of a file bind mount, once unmounted and
// left empty. A mount point the resource found in place is the operator's,
// and what it still holds, or a filesystem still mounted on it, is not the
// resource's to remove: they are left, with a log.
func (t *T) removeMountPoint() error {
	mnt := t.mountPoint()
	if mnt == "." || file.IsProtected(mnt) {
		return nil
	}
	if !t.createdMountPoint(mnt) {
		return nil
	}
	fi, err := os.Lstat(mnt)
	if errors.Is(err, os.ErrNotExist) {
		t.forgetMountPointCreated()
		return nil
	} else if err != nil {
		return err
	}
	if !fi.IsDir() && !(fi.Mode().IsRegular() && fi.Size() == 0) {
		return nil
	}
	remove := os.Remove
	if z, err := t.zoneOf(); err != nil {
		return err
	} else if z != nil {
		if z.IsRoot() {
			return nil
		}
		remove = func(string) error { return z.Remove() }
	}
	if err := remove(mnt); err != nil {
		t.Log().Infof("leave the mount point %s: %s", mnt, err)
		return nil
	}
	t.forgetMountPointCreated()
	t.Log().Infof("removed the mount point %s", mnt)
	return nil
}

// mountPointCreatedFile records the mount point a start of the resource
// created, which the unprovision may remove: its path, and the identity of
// the directory or the file it was, its device and its inode, for a path
// replaced since to be told apart.
func (t *T) mountPointCreatedFile() string {
	return filepath.Join(t.VarDir(), "mnt_created")
}

// mountPointIdentity returns the identity of what is at mnt, its device and
// its inode, which a path replaced does not keep.
func mountPointIdentity(mnt string) (string, error) {
	fi, err := os.Lstat(mnt)
	if err != nil {
		return "", err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("%s: no device and inode", mnt)
	}
	return fmt.Sprintf("%d %d", st.Dev, st.Ino), nil
}

// createdMountPoint says if what is at mnt is the mount point a start of the
// resource created: a change of mnt, or a path replaced since, as by an
// operator once the resource stopped, leaves it to the operator, and the
// record is forgotten.
func (t *T) createdMountPoint(mnt string) bool {
	b, err := os.ReadFile(t.mountPointCreatedFile())
	if err != nil {
		return false
	}
	recorded, identity, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	if recorded != mnt {
		return false
	}
	current, err := mountPointIdentity(mnt)
	if errors.Is(err, os.ErrNotExist) {
		t.forgetMountPointCreated()
		return false
	}
	if err != nil || current != identity {
		t.Log().Infof("leave the mount point %s: it is not the one the resource created", mnt)
		t.forgetMountPointCreated()
		return false
	}
	return true
}

func (t *T) markMountPointCreated(mnt string) {
	identity, err := mountPointIdentity(mnt)
	if err != nil {
		t.Log().Warnf("record the mount point %s created: %s", mnt, err)
		return
	}
	p := t.mountPointCreatedFile()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Log().Warnf("record the mount point %s created: %s", mnt, err)
		return
	}
	if err := os.WriteFile(p, []byte(mnt+"\n"+identity+"\n"), 0600); err != nil {
		t.Log().Warnf("record the mount point %s created: %s", mnt, err)
	}
}

func (t *T) forgetMountPointCreated() {
	if err := os.Remove(t.mountPointCreatedFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Log().Warnf("forget the mount point created: %s", err)
	}
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	fs := t.fs()
	v, err := filesystems.DevicesFormated(ctx, fs, t)
	if errors.Is(err, filesystems.ErrNotImplemented) {
		t.Log().Tracef("driver has not implemented IsFormatted")
		return provisioned.NotApplicable, nil
	} else if err != nil {
		t.Log().Tracef("IsFormatted: %s", err)
		return provisioned.Undef, err
	}
	t.Log().Tracef("IsFormatted: %v", v)
	if v {
		return provisioned.True, nil
	}
	return provisioned.False, nil
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "dev", Value: t.devpath(ctx)},
		{Key: "mnt", Value: t.mountPoint()},
		{Key: "mnt_opt", Value: t.MountOptions},
	}
	return m, nil
}

// StatusInfo implements resource.StatusInfoer
func (t *T) StatusInfo(ctx context.Context) map[string]interface{} {
	data := make(map[string]interface{})
	data["mnt"] = t.mountPoint()
	return data
}

func (t *T) testFile() string {
	return filepath.Join(t.mountPoint(), ".opensvc")
}

// mountOptions returns the options the filesystem is mounted with.
//
// The size and the mode of a tmpfs are options of its mount, and keywords of
// its resource: the size keyword is what a resize records the size it reached
// in, and a mount option is not. An option the keyword sets is dropped from
// mnt_opt, which would otherwise be read after it and win.
func (t *T) mountOptions() string {
	if t.isBtrfs() {
		return t.btrfsMountOptions()
	}
	if !t.isTmpfs() || (t.Size == nil && t.Mode == nil) {
		return t.MountOptions
	}
	l := make([]string, 0)
	for _, option := range strings.Split(t.MountOptions, ",") {
		switch {
		case option == "":
		case t.Size != nil && strings.HasPrefix(option, "size="):
		case t.Mode != nil && strings.HasPrefix(option, "mode="):
		default:
			l = append(l, option)
		}
	}
	if t.Size != nil {
		l = append(l, fmt.Sprintf("size=%d", *t.Size))
	}
	if t.Mode != nil {
		l = append(l, "mode="+octalMode(*t.Mode))
	}
	return strings.Join(l, ",")
}

// overriddenMountOptions returns the options of mnt_opt a keyword sets
// instead, which a mount ignores.
func (t *T) overriddenMountOptions() []string {
	if !t.isTmpfs() {
		return nil
	}
	l := make([]string, 0)
	for _, option := range strings.Split(t.MountOptions, ",") {
		switch {
		case t.Size != nil && strings.HasPrefix(option, "size="):
			l = append(l, option+" is ignored: the size keyword sets the size")
		case t.Mode != nil && strings.HasPrefix(option, "mode="):
			l = append(l, option+" is ignored: the mode keyword sets the mode")
		}
	}
	return l
}

func (t *T) isTmpfs() bool {
	return t.Type == "tmpfs"
}

// octalMode formats a mode the way a mount option takes it, the special bits
// included: 1777 is a sticky directory everyone may write in.
func octalMode(m os.FileMode) string {
	n := uint32(m.Perm())
	if m&os.ModeSetuid != 0 {
		n |= 04000
	}
	if m&os.ModeSetgid != 0 {
		n |= 02000
	}
	if m&os.ModeSticky != 0 {
		n |= 01000
	}
	return strconv.FormatUint(uint64(n), 8)
}

// mountPoint returns where the filesystem is mounted on the host: mnt,
// under the root directory of the container zone names when set. It is
// empty when the zone names no container, which resolveMountPoint says.
func (t *T) mountPoint() string {
	p, _ := t.resolveMountPoint()
	return p
}

// resolveMountPoint returns the mount point, or why the zone the filesystem
// is mounted in can not be found.
//
// The path of a zoned mount point is checked at each call: a process of the
// container can make a component of it a link since the last one.
func (t *T) resolveMountPoint() (string, error) {
	z, err := t.zoneOf()
	if err != nil {
		return "", err
	}
	if z == nil {
		return filepath.Clean(t.MountPoint), nil
	}
	if err := z.Check(); err != nil {
		return "", err
	}
	return z.Path(), nil
}

// zoneOf returns the mount point in the root directory of the container the
// zone keyword names, nil when it is not set.
func (t *T) zoneOf() (*Zone, error) {
	if t.Zone == "" {
		return nil, nil
	}
	if t.zone == nil {
		z, err := ZoneOf(context.Background(), t.GetObjectDriver(), t.Zone, t.MountPoint)
		if err != nil {
			return nil, err
		}
		t.zone = &z
	}
	return t.zone, nil
}

// mountFS mounts the filesystem. A zoned one is mounted on the mount point
// opened through the root of the container, never on where its path leads
// by then.
func (t *T) mountFS(ctx context.Context) error {
	z, err := t.zoneOf()
	if err != nil {
		return err
	}
	if z == nil {
		return t.fs().Mount(ctx, t.devpath(ctx), t.mountPoint(), t.mountOptions())
	}
	f, err := z.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	return t.fs().MountNoCanonicalize(ctx, t.devpath(ctx), FDPath(f), t.mountOptions())
}

// umountFS unmounts the filesystem. A zoned one is unmounted through the
// parent of its mount point opened through the root of the container, a
// link swapped in at the mount point not followed.
func (t *T) umountFS(ctx context.Context) error {
	z, err := t.zoneOf()
	if err != nil {
		return err
	}
	if z == nil {
		return t.fs().Umount(ctx, t.mountPoint())
	}
	target, parent, err := z.UmountTarget()
	if err != nil {
		return err
	}
	if parent != nil {
		defer parent.Close()
	}
	return t.fs().UmountNoFollow(ctx, target)
}

func (t *T) device(ctx context.Context) device.T {
	return device.New(t.devpath(ctx), device.WithLogger(t.Log()))
}

func (t *T) devpath(ctx context.Context) string {
	if t.fs().IsFileBacked() {
		return t.Device
	}
	if t.fs().IsNetworked() {
		return t.Device
	}
	if t.fs().IsVirtual() {
		return "none"
	}
	if t.hasMountOption("loop") {
		return t.Device
	}
	if p, err := vpath.Devpath(ctx, t.RID(), t.Device, t.Path.Namespace, vpath.ResolverOf(t.GetObject())); err == nil {
		return p
	} else {
		t.Log().Tracef("resolve host devpath for device %s in namespace %s: %s", t.Device, t.Path.Namespace, err)
	}
	return ""
}

func (t *T) mount(ctx context.Context) error {
	if _, err := t.resolveMountPoint(); err != nil {
		return err
	}
	if err := errors.Join(t.btrfsConflicts()...); err != nil {
		return err
	}
	if err := t.validateDevice(ctx); err != nil {
		return err
	}
	if err := t.promoteDevicesReadWrite(ctx); err != nil {
		return err
	}
	if v, err := t.isMounted(ctx); err != nil {
		return err
	} else if v {
		if devpath := t.devpath(ctx); devpath != "none" {
			t.Log().Infof("%s already mounted on %s", devpath, t.mountPoint())
		} else {
			t.Log().Infof("already mounted on %s", t.mountPoint())
		}
		return nil
	}
	if err := t.createDevice(ctx); err != nil {
		return err
	}
	if err := t.createMountPoint(ctx); err != nil {
		return err
	}
	if err := t.fsck(ctx); err != nil {
		return err
	}
	if err := t.mountFS(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.umountFS(ctx)
	})
	return nil
}

func (t *T) createDevice(ctx context.Context) error {
	p := t.devpath(ctx)
	fs := t.fs()
	if !fs.IsFileBacked() {
		return nil
	}
	if file.Exists(p) {
		return nil
	}
	t.Log().Infof("create missing device %s", p)
	if err := os.MkdirAll(p, defaultPerm); err != nil {
		return fmt.Errorf("error creating device %s: %s", p, err)
	}
	return nil
}

func (t *T) createMountPoint(ctx context.Context) error {
	_, lstatErr := os.Lstat(t.mountPoint())
	existed := lstatErr == nil
	isRegular, err := file.ExistsAndRegular(t.Device)
	if err != nil {
		return err
	}
	z, err := t.zoneOf()
	if err != nil {
		return err
	}
	switch {
	case z != nil && isRegular && t.isBind():
		// A zoned mount point is made through the root of the
		// container, never out of it.
		t.Log().Infof("create missing mountpoint file %s", t.mountPoint())
		err = z.CreateFile(t.mountPointDirPerm())
	case z != nil:
		t.Log().Infof("create missing mountpoint dir %s", t.mountPoint())
		err = z.MkdirAll(t.mountPointDirPerm())
	case isRegular && t.isBind():
		err = t.createMountPointFile()
	default:
		err = t.createMountPointDir(t.mountPoint())
	}
	if err != nil {
		return err
	}
	if !existed {
		t.markMountPointCreated(t.mountPoint())
	}
	return nil
}

// createMountPointFile handles the creation of the hosting dir and mnt
// file for a file bind mount like:
//
//	[fs#1]
//	type = bind
//	dev = /a/foo
//	mnt = /b/foo
func (t *T) createMountPointFile() error {
	dir := filepath.Dir(t.mountPoint())
	if err := t.createMountPointDir(dir); err != nil {
		return err
	}

	if stat, err := os.Stat(t.mountPoint()); err == nil {
		// file already exists
		if stat.Mode().IsRegular() {
			return nil
		} else {
			return fmt.Errorf("the mountpoint already exists but is not a regular file")
		}
	} else if os.IsNotExist(err) {
		t.Log().Infof("create missing mountpoint file %s", t.mountPoint())
		// Exclusive, so a symlink made at the mount point since the stat is
		// not followed to a file created elsewhere.
		if f, err := os.OpenFile(t.mountPoint(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644); err != nil {
			return err
		} else {
			f.Close()
			return nil
		}
	} else {
		return err
	}
}

func (t *T) createMountPointDir(dir string) error {
	if v, err := file.ExistsAndDir(dir); err != nil {
		return err
	} else if v {
		return nil
	}
	if file.Exists(dir) {
		return fmt.Errorf("mountpoint %s already exists but is not a directory", dir)
	}

	t.Log().Infof("create missing mountpoint dir %s", dir)
	if err := os.MkdirAll(dir, t.mountPointDirPerm()); err != nil {
		return fmt.Errorf("error creating mountpoint %s: %w", dir, err)
	}
	return nil
}

// mountPointDirPerm is the mode of the mount point directories the resource
// makes.
func (t *T) mountPointDirPerm() os.FileMode {
	if p := t.DataRecv.RootDirPerm(); p != nil {
		return *p
	}
	return defaultPerm
}

func (t *T) validateDevice(ctx context.Context) error {
	fs := t.fs()
	if fs.IsZero() {
		return nil
	}
	if fs.IsMultiDevice() {
		return nil
	}
	if fs.IsVirtual() {
		return nil
	}
	if t.Device == "" {
		return fmt.Errorf("device keyword not set or evaluates to None")
	}
	if t.isByLabel() {
		return nil
	}
	if t.isByUUID() {
		return nil
	}
	dev := t.devpath(ctx)
	if (!fs.IsFileBacked() && !fs.IsNetworked()) && !file.Exists(dev) {
		return fmt.Errorf("device does not exist: %s", dev)
	}
	return nil
}

func (t *T) isByUUID() bool {
	return strings.HasPrefix(t.Device, "UUID=")
}

func (t *T) isByLabel() bool {
	return strings.HasPrefix(t.Device, "LABEL=")
}

func (t *T) ClaimedDevices(ctx context.Context) device.L {
	return t.SubDevices(ctx)
}

func (t *T) ReservableDevices(ctx context.Context) device.L {
	return t.SubDevices(ctx)
}

func (t *T) SubDevices(ctx context.Context) device.L {
	l := make(device.L, 0)
	fs := t.fs()
	if !fs.IsMultiDevice() {
		l = append(l, t.device(ctx))
		return l
	}
	i, ok := fs.(filesystems.DeviceLister)
	if !ok {
		t.Log().Warnf("TODO: multi dev SubDevices()")
		return l
	}
	devpath := t.devpath(ctx)
	if devpath == "" {
		return l
	}
	paths, err := i.Devices(ctx, devpath)
	if err != nil {
		t.Log().Debugf("list the devices of the %s on %s: %s", fs, devpath, err)
		return append(l, t.device(ctx))
	}
	for _, p := range paths {
		l = append(l, device.New(p, device.WithLogger(t.Log())))
	}
	return l
}

func (t *T) promoteDevicesReadWrite(ctx context.Context) error {
	if !t.PromoteRW {
		return nil
	}
	devices := t.SubDevices(ctx)
	for _, dev := range devices {
		currentRO, err := dev.IsReadOnly()
		if err != nil {
			return err
		}
		if !currentRO {
			t.Log().Tracef("device %s is already read-write", dev)
			continue
		}
		t.Log().Infof("promote device %s read-write", dev)
		if err := dev.SetReadWrite(ctx); err != nil {
			return err
		}
		actionrollback.Register(ctx, func(ctx context.Context) error {
			return dev.SetReadOnly(ctx)
		})
	}
	return nil
}

func (t *T) fs() filesystems.I {
	fs := filesystems.FromType(t.Type)
	fs.SetLog(t.Log())
	return fs
}

func (t *T) fsck(ctx context.Context) error {
	fs := t.fs()
	if !filesystems.HasFSCK(fs) {
		t.Log().Tracef("skip fsck, not implemented for type %s", fs)
		return nil
	}
	if err := filesystems.CanFSCK(fs); err != nil {
		t.Log().Warnf("skip fsck: %s", err)
		return nil
	}
	return filesystems.DevicesFSCK(ctx, fs, t)
}

func (t *T) isBindMounted(ctx context.Context) (bool, error) {
	isMounted, err := findmnt.HasMnt(ctx, t.mountPoint())
	if err != nil || !isMounted {
		return isMounted, err
	}

	// We know we have a entry in findmnt, but not yet sure it's our bind mount.
	// Verify the dev and mnt have the same maj:min and inode.
	err = file.VerifySameMajorMinorAndInode(t.Device, t.mountPoint())
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if errors.Is(err, file.ErrNotSame) {
		t.StatusLog().Info("%s", err)
		return false, nil
	} else if err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func (t *T) isBind() bool {
	return t.hasMountOption("bind") || t.Type == "bind"
}

// isMounted says if the filesystem is mounted on its mount point. A mount
// point that does not resolve, as a zoned one with a link on its path, is an
// error: asked about an empty path, findmnt could answer with any mount.
func (t *T) isMounted(ctx context.Context) (bool, error) {
	mnt, err := t.resolveMountPoint()
	if err != nil {
		return false, err
	}
	if t.hasMountOption("loop") {
		return findmnt.HasFromMount(t.devpath(ctx), mnt)
	}
	if t.isBind() {
		return t.isBindMounted(ctx)
	}
	if t.fs().IsVirtual() {
		return findmnt.HasMntWithTypes(ctx, []string{t.Type}, mnt)
	}
	return findmnt.Has(ctx, t.devpath(ctx), mnt)
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	if _, err := t.resolveMountPoint(); err != nil {
		return err
	}
	if err := errors.Join(t.btrfsConflicts()...); err != nil {
		return err
	}
	if err := t.mkfs(ctx); err != nil {
		return err
	}
	// The parts of the filesystem the mount options name, as the
	// subvolume of a btrfs, are created on a filesystem already formatted
	// too: the mount needs them.
	if i, ok := t.fs().(filesystems.MountOptionsProvisioner); ok {
		devpath := t.devpath(ctx)
		if devpath == "" {
			return fmt.Errorf("%s real dev path is empty", t.Device)
		}
		return i.ProvisionMountOptions(ctx, devpath, t.mountOptions())
	}
	return nil
}

func (t *T) mkfs(ctx context.Context) error {
	fs := t.fs()
	i1, ok := fs.(IsFormateder)
	if !ok {
		t.Log().Infof("skip mkfs, formatted detection is not implemented for type %s", fs)
		return nil
	}
	devpath := t.devpath(ctx)
	if devpath == "" {
		return fmt.Errorf("%s real dev path is empty", t.Device)
	}
	if v, err := i1.IsFormated(ctx, devpath); err != nil {
		t.Log().Warnf("skip mkfs: %s", err)
	} else if v {
		t.Log().Infof("%s is already formatted", t.Device)
		return nil
	}
	i2, ok := fs.(filesystems.MKFSer)
	if ok {
		return i2.MKFS(ctx, devpath, t.mkfsOptions())
	}
	t.Log().Infof("skip mkfs, not implemented for type %s", fs)
	return nil
}

func (t *T) Head() string {
	return t.mountPoint()
}

func (t *T) canCheckWriteAccess() bool {
	fs := t.fs()
	if fs.IsNetworked() || fs.IsReadOnly() || t.hasMountOption("ro") || fs.IsVirtual() || fs.Type() == "bind" {
		return false
	}
	return true
}

// checkWriteAccess returns nil if we can write to mount point.
// It uses setxattr on mountpoint, fallback to write file '.opensvc' in mountpoint.
func (t *T) checkWriteAccess() error {
	mountPoint := t.mountPoint()
	if err := t.checkWriteXattr(mountPoint); err != nil {
		if err = t.checkWriteFile(path.Join(mountPoint, ".opensvc")); err != nil {
			return fmt.Errorf("check write access: %w", err)
		}
	}
	return nil
}

func (t *T) canCheckReadAccess() bool {
	if !t.CheckRead {
		return false
	}
	if t.hasMountOption("nointr") {
		return false
	}
	return true
}

func (t *T) checkReadAccess() error {
	var (
		cmd  *exec.Cmd
		name = "stat"
		arg  = []string{"-f", t.mountPoint()}
		now  = time.Now()
	)
	if t.StatTimeout != nil {
		ctx, cancel := context.WithTimeout(context.Background(), *t.StatTimeout)
		defer cancel()
		cmd = exec.CommandContext(ctx, name, arg...)
	} else {
		cmd = exec.Command(name, arg...)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("check read access failed after %d ms: %w", time.Now().Sub(now).Milliseconds(), err)
	}
	return nil
}

func (t *T) checkWriteXattr(s string) error {
	data := []byte(time.Now().String())
	t.Log().Tracef("checkWriteXattr %s", s)
	return unix.Setxattr(s, "user.opensvc", data, 0)
}

// checkWriteFile writes a byte in the file s of the mounted filesystem.
//
// What the filesystem holds is its users', who can make s a symlink to a file
// of the node, or a fifo: the probe, run as root by every status, follows no
// symlink, waits on no reader, writes only in a regular file, and truncates
// nothing.
func (t *T) checkWriteFile(s string) error {
	t.Log().Tracef("checkWriteFile %s", s)
	f, err := os.OpenFile(s, os.O_CREATE|os.O_WRONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	if fi, err := f.Stat(); err != nil {
		return err
	} else if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", s)
	}
	if _, err := f.Write([]byte(" ")); err != nil {
		return err
	}
	return nil
}

func (t *T) hasMountOption(option string) bool {
	for _, s := range strings.Split(t.mountOptions(), ",") {
		if s == option {
			return true
		}
	}
	return false
}

func (t *T) CanInstall(ctx context.Context) (bool, error) {
	state := t.Status(ctx)
	if state != status.Up {
		return false, nil
	}
	return true, nil
}

// CurrentSize implements resource.Sizer.
//
// It is what the filesystem hands out, which is less than the device under it
// holds: it keeps metadata and a log for itself.
//
// Reporting the device instead would read better on a converged chain, where
// a 5.49gi device carrying an xfs that hands out 5.43gi looks like 60mi still
// to grow. It is not worth it: the size would then equal the target the
// moment the device grew, and the step that grows the filesystem onto it
// would be skipped as already done. A filesystem short of its device is the
// case this has to be able to see.
func (t *T) CurrentSize(ctx context.Context) (int64, error) {
	if _, ok := t.fs().(filesystems.SelfSizer); ok {
		// A filesystem holding its own size, a tmpfs, has no device to
		// measure, and what it hands out is what it was given.
		return t.usableSize(ctx)
	}
	dev := t.devpath(ctx)
	if dev == "" {
		return t.usableSize(ctx)
	}
	// The mount is checked first. A filesystem is the size of the device it
	// was given, but an unmounted resource is not holding that device, and
	// saying it is would resolve a relative resize against a size nothing
	// here has.
	if _, err := t.usableSize(ctx); err != nil {
		return 0, err
	}
	return device.New(dev).Size()
}

// ResizeSpansBelow implements resource.ResizeSpansBelow. A filesystem is
// grown onto the device under it, not to a size of its own: Grow is told the
// device and the mount point, and nothing else.
func (t *T) ResizeSpansBelow() bool {
	_, ok := t.fs().(filesystems.SelfSizer)
	return !ok
}

// usableSize is what the filesystem hands out, which is less than the device
// holds: it keeps metadata and a log for itself.
//
// It is what says whether the filesystem has yet been grown onto its device,
// which the device size cannot: a chain grows from the bottom up, so by the
// time the filesystem is asked to grow, the device under it already holds the
// new size.
func (t *T) usableSize(ctx context.Context) (int64, error) {
	mnt := t.mountPoint()
	mounts, err := findmnt.List(ctx, "", mnt)
	if err != nil {
		return 0, err
	}
	if len(mounts) == 0 {
		return 0, fmt.Errorf("%s is not mounted on %s, so its size cannot be read", t.Device, mnt)
	}
	var st unix.Statfs_t
	if err := unix.Statfs(mnt, &st); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", mnt, err)
	}
	return int64(st.Blocks) * int64(st.Bsize), nil
}

// ResizePlan implements resource.Resizer.
//
// A filesystem takes up the device it sits on, so it asks the link below for
// the size it was asked for. It refuses here rather than in Resize: a
// filesystem that cannot shrink must say so before the device under it is
// taken away.
func (t *T) ResizePlan(ctx context.Context, to int64) (int64, error) {
	fs := t.fs()
	from, err := t.CurrentSize(ctx)
	if err != nil {
		return 0, err
	}
	if _, ok := fs.(filesystems.SelfSizer); ok {
		// A filesystem holding its own size asks nothing of anything below
		// it, and there is nothing below it to ask.
		return to, nil
	}
	if to < from {
		// A resize only grows, and the plan says so before reaching here.
		return from, nil
	}
	if _, ok := fs.(filesystems.Grower); !ok {
		return 0, fmt.Errorf("a %s filesystem cannot grow", fs.Type())
	}
	return to, nil
}

// Resize implements resource.Resizer.
func (t *T) Resize(ctx context.Context, to int64) error {
	fs := t.fs()
	if selfSizer, ok := fs.(filesystems.SelfSizer); ok {
		return selfSizer.SetSize(ctx, t.mountPoint(), to)
	}
	grower, ok := fs.(filesystems.Grower)
	if !ok {
		return fmt.Errorf("a %s filesystem cannot grow", fs.Type())
	}
	// The device below has been enlarged already, so the filesystem is asked
	// to take up what is there rather than a size of its own.
	return grower.Grow(ctx, t.devpath(ctx), t.mountPoint())
}
