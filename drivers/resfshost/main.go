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
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/drivers/resfsdir"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/filesystems"
	"github.com/opensvc/om3/v3/util/findmnt"
)

type (
	T struct {
		resource.T
		resource.SCSIPersistentReservation
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
	}

	IsFormateder interface {
		IsFormated(string) (bool, error)
	}
	MKFSer interface {
		MKFS(string, []string) error
	}
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

func (t *T) Start(ctx context.Context) error {
	if err := t.mount(ctx); err != nil {
		return err
	}
	if err := t.fsDir().Start(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if v, err := t.isMounted(); err != nil {
		return err
	} else if !v {
		t.Log().Infof("%s already umounted from %s", t.devpath(), t.mountPoint())
		return nil
	}
	mnt := t.mountPoint()
	fs := t.fs()
	if err := fs.Umount(mnt); err != nil {
		if file.IsProtected(mnt) {
			return err
		}
		if errors.Is(err, syscall.EBUSY) {
			for _ = range 4 {
				fs.KillUsers(mnt)
				if err := fs.Umount(mnt); err == nil {
					return nil
				}
			}
		}
		return err
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	if t.Device == "" {
		t.StatusLog().Info("dev is not defined")
		return status.NotApplicable
	}
	if t.MountPoint == "" {
		t.StatusLog().Info("mnt is not defined")
		return status.NotApplicable
	}
	if v, err := t.isMounted(); err != nil {
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
	return status.Up
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	s := t.devpath()
	m := t.mountPoint()
	if m != "" {
		s += "@" + m
	}
	return s
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "dev", Value: t.devpath()},
		{Key: "mnt", Value: t.mountPoint()},
		{Key: "mnt_opt", Value: t.MountOptions},
	}
	return m, nil
}

func (t *T) fsDir() *resfsdir.T {
	r := resfsdir.New().(*resfsdir.T)
	r.SetRID(t.RID())
	r.SetObject(t.GetObject())
	r.Path = t.MountPoint
	r.User = t.User
	r.Group = t.Group
	r.Perm = t.Perm
	return r
}

func (t *T) testFile() string {
	return filepath.Join(t.mountPoint(), ".opensvc")
}

func (t *T) mountOptions() string {
	// in can we need to mangle options
	return t.MountOptions
}

func (t *T) mountPoint() string {
	// add zonepath translation, and cache ?
	return filepath.Clean(t.MountPoint)
}

func (t *T) device() device.T {
	return device.New(t.devpath(), device.WithLogger(t.Log()))
}

func (t *T) devpath() string {
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
	if p, err := vpath.HostDevpath(t.Device, t.Path.Namespace); err == nil {
		return p
	} else {
		t.Log().Tracef("resolve host devpath for device %s in namespace %s: %s", t.Device, t.Path.Namespace, err)
	}
	return ""
}

func (t *T) mount(ctx context.Context) error {
	if err := t.validateDevice(); err != nil {
		return err
	}
	if err := t.promoteDevicesReadWrite(ctx); err != nil {
		return err
	}
	if v, err := t.isMounted(); err != nil {
		return err
	} else if v {
		t.Log().Infof("%s already mounted on %s", t.devpath(), t.mountPoint())
		return nil
	}
	if err := t.createDevice(ctx); err != nil {
		return err
	}
	if err := t.createMountPoint(ctx); err != nil {
		return err
	}
	if err := t.fsck(); err != nil {
		return err
	}
	if err := t.fs().Mount(t.devpath(), t.mountPoint(), t.mountOptions()); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.fs().Umount(t.mountPoint())
	})
	return nil
}

func (t *T) createDevice(ctx context.Context) error {
	p := t.devpath()
	fs := t.fs()
	if !fs.IsFileBacked() {
		return nil
	}
	if file.Exists(p) {
		return nil
	}
	t.Log().Infof("create missing device %s", p)
	if err := os.MkdirAll(p, 0755); err != nil {
		return fmt.Errorf("error creating device %s: %s", p, err)
	}
	return nil
}

func (t *T) createMountPoint(ctx context.Context) error {
	if v, err := file.ExistsAndDir(t.MountPoint); err != nil {
		return err
	} else if v {
		return nil
	}
	if file.Exists(t.MountPoint) {
		return fmt.Errorf("mountpoint %s already exists but is not a directory", t.MountPoint)
	}
	t.Log().Infof("create missing mountpoint %s", t.MountPoint)
	if err := os.MkdirAll(t.MountPoint, 0755); err != nil {
		return fmt.Errorf("error creating mountpoint %s: %s", t.MountPoint, err)
	}
	return nil
}

func (t *T) validateDevice() error {
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
	dev := t.devpath()
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

func (t *T) ClaimedDevices() device.L {
	return t.SubDevices()
}

func (t *T) ReservableDevices() device.L {
	return t.SubDevices()
}

func (t *T) SubDevices() device.L {
	l := make(device.L, 0)
	fs := t.fs()
	if !fs.IsMultiDevice() {
		l = append(l, t.device())
		return l
	}
	t.Log().Warnf("TODO: multi dev SubDevices()")
	return l
}

func (t *T) promoteDevicesReadWrite(ctx context.Context) error {
	if !t.PromoteRW {
		return nil
	}
	devices := t.SubDevices()
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
		if err := dev.SetReadWrite(); err != nil {
			return err
		}
		actionrollback.Register(ctx, func(ctx context.Context) error {
			return dev.SetReadOnly()
		})
	}
	return nil
}

func (t *T) fs() filesystems.I {
	fs := filesystems.FromType(t.Type)
	fs.SetLog(t.Log())
	return fs
}

func (t *T) fsck() error {
	fs := t.fs()
	if !filesystems.HasFSCK(fs) {
		t.Log().Tracef("skip fsck, not implemented for type %s", fs)
		return nil
	}
	if err := filesystems.CanFSCK(fs); err != nil {
		t.Log().Warnf("skip fsck: %s", err)
		return nil
	}
	return filesystems.DevicesFSCK(fs, t)
}

func (t *T) isMounted() (bool, error) {
	if t.hasMountOption("loop") {
		return findmnt.HasFromMount(t.devpath(), t.mountPoint())
	}
	if t.Type == "tmpfs" {
		return findmnt.HasMntWithTypes([]string{"tmpfs"}, t.mountPoint())
	}
	return findmnt.Has(t.devpath(), t.mountPoint())
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	fs := t.fs()
	i1, ok := fs.(IsFormateder)
	if !ok {
		t.Log().Infof("skip mkfs, formatted detection is not implemented for type %s", fs)
		return nil
	}
	devpath := t.devpath()
	if devpath == "" {
		return fmt.Errorf("%s real dev path is empty", t.Device)
	}
	if v, err := i1.IsFormated(devpath); err != nil {
		t.Log().Warnf("skip mkfs: %s", err)
	} else if v {
		t.Log().Infof("%s is already formatted", t.Device)
		return nil
	}
	i2, ok := fs.(filesystems.MKFSer)
	if ok {
		return i2.MKFS(t.Device, t.MKFSOptions)
	}
	t.Log().Infof("skip mkfs, not implemented for type %s", fs)
	return nil
}

func (t *T) Head() string {
	return t.MountPoint
}

func (t *T) canCheckWriteAccess() bool {
	if t.fs().IsNetworked() || t.hasMountOption("ro") {
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

func (t *T) checkWriteFile(s string) error {
	t.Log().Tracef("checkWriteFile %s", s)
	if f, err := os.OpenFile(s, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600); err != nil {
		return err
	} else {
		defer func() {
			_ = f.Close()
		}()
		if _, err := f.Write([]byte(" ")); err != nil {
			return err
		}
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
