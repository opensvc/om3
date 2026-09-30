package resfszfs

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
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
		resource.Restart
		datarecv.DataRecv
		MountPoint     string         `json:"mnt"`
		Device         string         `json:"dev"`
		MountOptions   string         `json:"mnt_opt"`
		StatTimeout    *time.Duration `json:"stat_timeout"`
		Size           *int64         `json:"size"`
		Zone           string         `json:"zone"`
		MKFSOptions    []string       `json:"mkfs_opt"`
		RefQuota       string         `json:"refquota"`
		Quota          string         `json:"quota"`
		RefReservation string         `json:"refreservation"`
		Reservation    string         `json:"reservation"`
	}
)

const (
	defaultPerm = 0755
)

func New() resource.Driver {
	t := &T{}
	return t
}

// Configure installs a resource backpointer in the DataStoreInstall
func (t *T) Configure() error {
	t.DataRecv.SetReceiver(t)
	return nil
}

func (t *T) CanInstall(ctx context.Context) (bool, error) {
	state := t.Status(ctx)
	if state != status.Up {
		return false, nil
	}
	return true, nil
}

func (t *T) Start(ctx context.Context) error {
	if err := t.mount(ctx); err != nil {
		return err
	}
	if err := t.DataRecv.Do(ctx); err != nil {
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
	t.DataRecv.Status()
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

// StatusInfo implements resource.StatusInfoer
func (t *T) StatusInfo(ctx context.Context) map[string]interface{} {
	data := make(map[string]interface{})
	data["mnt"] = t.mountPoint()
	return data
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
		a.Append("-o", strings.Join(mountOptions, ","))
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
	var perm os.FileMode
	if p := t.DataRecv.RootDirPerm(); p != nil {
		perm = *p
	} else {
		perm = defaultPerm
	}

	if err := os.MkdirAll(t.MountPoint, perm); err != nil {
		return fmt.Errorf("error creating mountpoint %s: %s", t.MountPoint, err)
	}
	return nil
}

// CurrentSize implements resource.Sizer.
//
// A dataset is bounded by its refquota, which is what it may hold of its own
// and what df reports for it, or by its quota when it has no refquota. One
// with neither takes what the pool has, and has no size to report.
func (t *T) CurrentSize(ctx context.Context) (int64, error) {
	cur, err := t.datasetSizeProperties()
	if err != nil {
		return 0, err
	}
	size, _, err := datasetSize(cur, t.sizeExpressions())
	if err != nil {
		return 0, fmt.Errorf("%s %w", t.Device, err)
	}
	return size, nil
}

// ResizePlan implements resource.Resizer.
//
// A dataset takes its space from the pool holding it, which hands out what it
// has, so there is nothing below to ask a size of. A refquota under what the
// dataset already holds, or a quota under what it holds with its descendants
// and snapshots, is refused here: lowering them does not fail, it silently
// breaks the next write.
func (t *T) ResizePlan(ctx context.Context, to int64) (int64, error) {
	cur, err := t.datasetSizeProperties()
	if err != nil {
		return 0, err
	}
	next, err := resizedProperties(cur, t.sizeExpressions(), to)
	if err != nil {
		return 0, fmt.Errorf("%s %w", t.Device, err)
	}
	for prop, holds := range map[string]string{"refquota": "referenced", "quota": "used"} {
		v, ok := next[prop]
		if !ok {
			continue
		}
		used, err := t.datasetProperty(holds)
		if err != nil {
			return 0, err
		}
		if v < used {
			return 0, fmt.Errorf("%s already holds %s: a %s under that does not fail, it breaks the next write",
				t.Device, sizeconv.BSizeCompact(float64(used)), prop)
		}
	}
	return to, nil
}

// Resize implements resource.Resizer.
//
// The properties bounding the dataset move together: a resize moving the
// refquota alone would leave a quota capping the dataset under the size it
// was given, and a reservation promising less than it.
func (t *T) Resize(ctx context.Context, to int64) error {
	cur, err := t.datasetSizeProperties()
	if err != nil {
		return err
	}
	next, err := resizedProperties(cur, t.sizeExpressions(), to)
	if err != nil {
		return fmt.Errorf("%s %w", t.Device, err)
	}
	size, _, err := datasetSize(cur, t.sizeExpressions())
	if err != nil {
		return fmt.Errorf("%s %w", t.Device, err)
	}
	fs := t.fs()
	for _, prop := range sizePropertiesOrder(to >= size) {
		v, ok := next[prop]
		if !ok {
			continue
		}
		if err := fs.SetProperty(prop, fmt.Sprintf("%d", v)); err != nil {
			return err
		}
	}
	return nil
}

// sizeProperties are the dataset properties a size bounds, in the order a
// grow sets them: the caps are raised before what they cap, and a guarantee
// is raised last, once the cap over it has room for it.
var sizeProperties = []string{"quota", "refquota", "reservation", "refreservation"}

// sizePropertiesOrder is the order to set the properties in: a shrink lowers
// them in the reverse order of a grow, so a cap is never under what it caps.
func sizePropertiesOrder(grow bool) []string {
	l := slices.Clone(sizeProperties)
	if !grow {
		slices.Reverse(l)
	}
	return l
}

// sizeExpressions are the keywords of the size properties, as configured.
func (t *T) sizeExpressions() map[string]string {
	return map[string]string{
		"refquota":       t.refQuotaExpression(),
		"quota":          t.Quota,
		"refreservation": t.RefReservation,
		"reservation":    t.Reservation,
	}
}

// datasetSizeProperties reads the size properties of the dataset, 0 for one
// that is not set.
func (t *T) datasetSizeProperties() (map[string]int64, error) {
	cur := make(map[string]int64)
	for _, prop := range sizeProperties {
		v, err := t.datasetProperty(prop)
		if err != nil {
			return nil, err
		}
		cur[prop] = v
	}
	return cur, nil
}

// sizeMultiplier returns the N of a keyword written "xN", a multiplier of
// the size.
func sizeMultiplier(expr string) (float64, bool) {
	if !strings.HasPrefix(expr, "x") {
		return 0, false
	}
	n, err := strconv.ParseFloat(strings.TrimLeft(expr, "x"), 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// datasetSize returns the size a dataset holds, and the property that says
// it: the refquota, or the quota of a dataset with no refquota. A property
// configured as a multiplier of the size says the size divided by it.
func datasetSize(cur map[string]int64, exprs map[string]string) (int64, string, error) {
	for _, prop := range []string{"refquota", "quota"} {
		v := cur[prop]
		if v == 0 {
			continue
		}
		if n, ok := sizeMultiplier(exprs[prop]); ok {
			v = int64(math.Round(float64(v) / n))
		}
		return v, prop, nil
	}
	return 0, "", errors.New("has no refquota and no quota, so it takes what the pool has and has no size of its own")
}

// resizedProperties returns the properties to set for a dataset to hold the
// size asked, among the ones it has set.
//
// A property configured as a multiplier of the size is that multiple of the
// new size. One that was the size itself moves with it: a quota capping the
// dataset and its descendants at the size, a reservation guaranteeing the
// whole of it. One set to a size of its own is left alone, and the resize is
// refused if that leaves a quota under the refquota: the dataset would be
// given a size its quota does not let it reach.
func resizedProperties(cur map[string]int64, exprs map[string]string, to int64) (map[string]int64, error) {
	size, anchor, err := datasetSize(cur, exprs)
	if err != nil {
		return nil, err
	}
	next := make(map[string]int64)
	final := maps.Clone(cur)
	for _, prop := range sizeProperties {
		v := cur[prop]
		if v == 0 {
			continue
		}
		if n, ok := sizeMultiplier(exprs[prop]); ok {
			final[prop] = int64(float64(to) * n)
		} else if prop == anchor || v == size {
			final[prop] = to
		}
		if final[prop] != v {
			next[prop] = final[prop]
		}
	}
	if quota, refquota := final["quota"], final["refquota"]; quota > 0 && refquota > 0 && quota < refquota {
		return nil, fmt.Errorf("has a quota of %s, a size of its own under the %s asked: raise it, or configure it as a multiplier of size",
			sizeconv.BSizeCompact(float64(quota)), sizeconv.BSizeCompact(float64(refquota)))
	}
	return next, nil
}

// datasetProperty reads a byte-valued property, where zfs answers "none" or
// "-" for one that is not set.
func (t *T) datasetProperty(prop string) (int64, error) {
	s, err := t.fs().GetProperty(prop)
	if err != nil {
		return 0, err
	}
	switch s {
	case "", "none", "-", "0":
		return 0, nil
	}
	size, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: parse %s %s: %w", t.Device, prop, s, err)
	}
	return size, nil
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

// refQuotaExpression is the refquota keyword, x1 when it is not set and the
// size is: a dataset given a size is bounded by it.
func (t *T) refQuotaExpression() string {
	if t.RefQuota == "" && t.Size != nil {
		return "x1"
	}
	return t.RefQuota
}

func (t *T) refquota() (*int64, error) {
	return parseNoneOrFactorOrSize(t.Size, t.refQuotaExpression())
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
