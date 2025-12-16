package resfszfs

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resfsdir"
	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/findmnt"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/zfs"
)

type (
	T struct {
		resource.T
		MountPoint     string         `json:"mnt"`
		Device         string         `json:"dev"`
		MountOptions   string         `json:"mnt_opt"`
		StatTimeout    *time.Duration `json:"stat_timeout"`
		Size           *int64         `json:"size"`
		Zone           string         `json:"zone"`
		MKFSOptions    []string       `json:"mkfs_opt"`
		User           *user.User     `json:"user"`
		Group          *user.Group    `json:"group"`
		Perm           *os.FileMode   `json:"perm"`
		RefQuota       string         `json:"refquota"`
		Quota          string         `json:"quota"`
		RefReservation string         `json:"refreservation"`
		Reservation    string         `json:"reservation"`
	}
)

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
	if v, err := t.isMounted(ctx); err != nil {
		return err
	} else if !v {
		t.Log().Infof("%s already umounted from %s", t.Device, t.mountPoint())
		return nil
	}
	if err := t.umount(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) umount(ctx context.Context) error {
	if legacy, err := t.isLegacy(); err != nil {
		return err
	} else if err := t.umountWithLegacy(legacy); err != nil {
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
	if v, err := t.isMounted(ctx); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if !v {
		return status.Down
	}
	return status.Up
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	s := t.Device
	m := t.mountPoint()
	if m != "" {
		s += "@" + m
	}
	return s
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "dev", Value: t.Device},
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

func (t *T) mountOptions() []string {
	return strings.Split(t.MountOptions, ",")
}

func (t *T) mountPoint() string {
	// add zonepath translation, and cache ?
	return filepath.Clean(t.MountPoint)
}

func (t *T) device() device.T {
	return device.New(t.Device, device.WithLogger(t.Log()))
}

func (t *T) mount(ctx context.Context) error {
	if err := t.validateDevice(); err != nil {
		return err
	}
	if v, err := t.isMounted(ctx); err != nil {
		return err
	} else if v {
		t.Log().Infof("%s already mounted on %s", t.Device, t.mountPoint())
		return nil
	}
	if err := t.createMountPoint(ctx); err != nil {
		return err
	}
	if legacy, err := t.isLegacy(); err != nil {
		return err
	} else if err := t.mountWithLegacy(legacy); err != nil {
		return err
	} else {
		actionrollback.Register(ctx, func(ctx context.Context) error {
			return t.umountWithLegacy(legacy)
		})
	}
	return nil
}

func (t *T) umountWithLegacy(legacy bool) error {
	if legacy {
		return t.umountLegacy()
	} else {
		return t.umountNative()
	}
}

func (t *T) mountWithLegacy(legacy bool) error {
	if legacy {
		return t.mountLegacy()
	} else {
		return t.mountNative()
	}
}

func (t *T) mountLegacy() error {
	a := args.New()
	a.Append("-t", "zfs")
	mountOptions := t.mountOptions()
	if len(mountOptions) > 0 {
		a.Append("-o")
		a.Append(t.mountOptions()...)
	}
	a.Append(t.Device, t.MountPoint)
	cmd := command.New(
		command.WithName("mount"),
		command.WithArgs(a.Get()),
		command.WithLogger(t.Log()),
		command.WithTimeout(time.Minute),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	exitCode := cmd.ExitCode()
	if exitCode != 0 {
		return fmt.Errorf("%s exit code %d", cmd, exitCode)
	}
	return nil
}

func (t *T) umountLegacy() error {
	cmd := command.New(
		command.WithName("umount"),
		command.WithVarArgs(t.MountPoint),
		command.WithLogger(t.Log()),
		command.WithTimeout(time.Minute),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	exitCode := cmd.ExitCode()
	if exitCode != 0 {
		return fmt.Errorf("%s exit code %d", cmd, exitCode)
	}
	return nil
}

func (t *T) maySetMountPointProperty() error {
	fs := t.fs()
	mnt := t.mountPoint()
	mntProp, err := fs.GetProperty("mountpoint")
	if err != nil {
		return err
	}
	if mntProp == mnt {
		return nil
	}
	return fs.SetProperty("mountpoint", mnt)
}

func (t *T) mountNative() error {
	if err := t.maySetMountPointProperty(); err != nil {
		return err
	}
	return t.fs().Mount()
}

func (t *T) umountNative() error {
	fs := t.fs()
	if err := fs.Umount(); err == nil {
		return nil
	}
	return fs.Umount(
		zfs.FilesystemUmountWithForce(true),
	)
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

func (t *T) fs() *zfs.Filesystem {
	return &zfs.Filesystem{
		Log:  t.Log().Attr("device", t.Device).WithPrefix(t.Log().Prefix() + t.Device + ": "),
		Name: t.Device,
	}
}

func (t *T) pool() *zfs.Pool {
	return &zfs.Pool{
		Log:  t.Log().Attr("device", t.Device).WithPrefix(t.Log().Prefix() + t.Device + ": "),
		Name: t.poolName(),
	}
}

func (t *T) poolName() string {
	return zfs.DatasetName(t.Device).PoolName()
}

func (t *T) baseName() string {
	return zfs.DatasetName(t.Device).BaseName()
}

func (t *T) validateDevice() error {
	if t.baseName() == "" {
		return fmt.Errorf("device keyword value must be formatted like <pool>/<ds>")
	}
	if v, err := t.fs().Exists(); err != nil {
		return fmt.Errorf("dataset %s existence validation error: %w", t.Device, err)
	} else if !v {
		return fmt.Errorf("dataset %s does not exist", t.Device)
	}
	return nil
}

func (t *T) isMounted(ctx context.Context) (bool, error) {
	v, err := findmnt.Has(ctx, t.Device, t.mountPoint())
	return v, err
}

func factor(size *int64, expr string) (*int64, error) {
	if size == nil {
		return nil, fmt.Errorf("can not multiply empty size")
	}
	expr = strings.TrimLeft(expr, "x")
	multiplier, err := strconv.ParseFloat(expr, 10)
	if err != nil {
		return nil, err
	}
	f := float64(*size) * multiplier
	i := int64(f)
	return &i, nil
}

func parseNoneOrFactorOrSize(size *int64, expr string) (*int64, error) {
	switch {
	case expr == "":
		return nil, nil
	case expr == "none":
		return nil, nil
	case strings.HasPrefix(expr, "x"):
		return factor(size, expr)
	default:
		i, err := sizeconv.FromSize(expr)
		if err != nil {
			return nil, err
		}
		return &i, nil
	}
}

func (t *T) refquota() (*int64, error) {
	return parseNoneOrFactorOrSize(t.Size, t.RefQuota)
}

func (t *T) quota() (*int64, error) {
	return parseNoneOrFactorOrSize(t.Size, t.Quota)
}

func (t *T) refreservation() (*int64, error) {
	return parseNoneOrFactorOrSize(t.Size, t.RefReservation)
}

func (t *T) reservation() (*int64, error) {
	return parseNoneOrFactorOrSize(t.Size, t.Reservation)
}

func (t *T) mkfsOptions() []string {
	a := args.New()
	a.Set(t.MKFSOptions)
	if !a.HasOption("-p") {
		a.Append("-p")
	}
	if !a.HasOptionAndMatchingValue("-o", "^mountpoint=") {
		a.Append("-o", "mountpoint="+t.mountPoint())
	}
	if !a.HasOptionAndMatchingValue("-o", "^canmount=") {
		a.Append("-o", "canmount=noauto")
	}
	return a.Get()
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	if v, err := t.fs().Exists(); err != nil {
		return fmt.Errorf("fs existence check: %w", err)
	} else if v {
		t.Log().Infof("dataset %s already exists", t.Device)
		return nil
	}
	fopts := make([]funcopt.O, 0)
	fopts = append(fopts, zfs.FilesystemCreateWithArgs(t.mkfsOptions()))
	if v, err := t.refquota(); err != nil {
		return fmt.Errorf("refquota: %w", err)
	} else {
		fopts = append(fopts, zfs.FilesystemCreateWithRefQuota(v))
	}
	if v, err := t.quota(); err != nil {
		return fmt.Errorf("quota: %w", err)
	} else {
		fopts = append(fopts, zfs.FilesystemCreateWithQuota(v))
	}
	if v, err := t.refreservation(); err != nil {
		return fmt.Errorf("refreservation: %w", err)
	} else {
		fopts = append(fopts, zfs.FilesystemCreateWithRefReservation(v))
	}
	if v, err := t.reservation(); err != nil {
		return fmt.Errorf("refreservation: %w", err)
	} else {
		fopts = append(fopts, zfs.FilesystemCreateWithReservation(v))
	}
	if err := t.fs().Create(fopts...); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	return nil
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	fs := t.fs()
	if v, err := fs.Exists(); err != nil {
		return err
	} else if !v {
		t.Log().Infof("dataset %s is already destroyed", t.Device)
		return nil
	}
	if err := fs.Destroy(zfs.FilesystemDestroyWithRemoveSnapshots(true)); err != nil {
		return err
	}
	if err := t.removeMountPoint(); err != nil {
		return err
	}
	return nil
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) removeMountPoint() error {
	mnt := t.mountPoint()
	if mnt == "" {
		return nil
	}
	if file.IsProtected(mnt) {
		return fmt.Errorf("dir %s is protected: refuse to remove", mnt)
	}
	if !file.Exists(mnt) {
		t.Log().Infof("dir %s is already removed", mnt)
		return nil
	}
	return os.RemoveAll(mnt)
}

func (t *T) isLegacy() (bool, error) {
	if mountpoint, err := t.getMountPointProperty(); err != nil {
		return false, err
	} else {
		return mountpoint == "legacy", nil
	}
}

func (t *T) getMountPointProperty() (string, error) {
	if val, err := t.fs().GetProperty("mountpoint"); err != nil {
		return "", err
	} else {
		return val, nil
	}
}

func (t *T) Head() string {
	return t.MountPoint
}

func (t *T) ClaimedDevices(ctx context.Context) device.L {
	return t.SubDevices(ctx)
}

func (t *T) SubDevices(ctx context.Context) device.L {
	devs, _ := t.pool().VDevDevices(ctx)
	return devs
}
