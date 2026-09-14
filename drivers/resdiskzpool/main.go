package resdiskzpool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/zfs"
)

type (
	T struct {
		resdisk.T
		Name          string   `json:"name"`
		Size          string   `json:"size"`
		CreateOptions []string `json:"create_options"`
		VDev          []string `json:"vdev"`
		Multihost     string   `json:"multihost"`
		Zone          string   `json:"zone"`
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) subDevsFilePath() string {
	return filepath.Join(t.VarDir(), "sub_devs")
}

func (t *T) ToSync(ctx context.Context) []string {
	return []string{
		t.subDevsFilePath(),
	}
}

func (t *T) PreSync(ctx context.Context) error {
	_, err := t.updateSubDevsFile(ctx)
	return err
}

func (t *T) updateSubDevsFile(ctx context.Context) ([]string, error) {
	if v, err := t.hasIt(ctx); err != nil {
		return nil, err
	} else if !v {
		return nil, nil
	}
	l, err := t.pool().VDevPaths(ctx)
	if err != nil {
		return nil, fmt.Errorf("update sub devs cache: %w", err)
	}
	if err := t.writeSubDevsFile(l); err != nil {
		return l, err
	}
	return l, nil
}

func (t *T) writeSubDevsFile(l []string) error {
	path := t.subDevsFilePath()
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return fmt.Errorf("open temp sub devs cache: %w", err)
	}
	enc := json.NewEncoder(f)
	err = enc.Encode(l)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("json encode in sub devs cache: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp sub devs cache: %w", err)
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("install sub devs cache: %w", err)
	}
	return nil
}

func (t *T) loadSubDevsFile() ([]string, error) {
	path := t.subDevsFilePath()
	l := make([]string, 0)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open sub devs cache: %w", err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	err = dec.Decode(&l)
	if err != nil {
		return nil, fmt.Errorf("decode sub devs cache: %w", err)
	}
	return l, nil
}

func (t *T) hasIt(ctx context.Context) (bool, error) {
	return t.pool().Exists(ctx)
}

func (t *T) poolListZDevs(ctx context.Context) ([]string, error) {
	if zvols, err := t.pool().ListVolumes(ctx); err != nil {
		return nil, err
	} else {
		return zvols.Paths(), nil
	}
}

func (t *T) setMultihost() error {
	if t.Multihost == "" {
		return nil
	}
	var value string
	switch t.Multihost {
	case "true":
		value = "on"
	case "false":
		value = "off"
	}
	pool := t.pool()
	current, err := pool.GetProperty("multihost")
	if err != nil {
		return err
	}
	if current == value {
		t.Log().Infof("multihost property is already %s", value)
		return nil
	}
	return t.pool().SetProperty("multihost", value)
}

func (t *T) Start(ctx context.Context) error {
	if v, err := t.isUp(ctx); err != nil {
		return err
	} else if v {
		t.Log().Infof("%s is already up", t.Label(ctx))
		return nil
	}
	if err := t.doHostID(); err != nil {
		return err
	}
	if err := t.poolImport(); err != nil {
		return err
	}
	if err := t.setMultihost(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.poolExport()
	})
	return nil
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "name", Value: t.Name},
	}
	return m, nil
}

func (t *T) doHostID() error {
	switch t.Multihost {
	case "", "false":
		return nil
	default:
		return t.genHostID()
	}
}

func (t *T) genHostID() error {
	if file.Exists("/etc/hostid") {
		return nil
	}
	p, err := exec.LookPath("zgenhostid")
	if err != nil {
		t.Log().Warnf("/etc/hostid does not exist and zgenhostid is not installed")
		return nil
	}
	cmd := command.New(
		command.WithName(p),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

// UnprovisionStop skips the normal pre-unprovision resource stop,
// because zfs can only destroy imported pools. The Unprovision func
// imports anyway, but if we don't export unnecessary export/import is
// saved.
func (t *T) UnprovisionStop(ctx context.Context, leader bool) error {
	if !leader {
		t.Log().Debugf("stop for unprovision leader")
		return t.Stop(ctx)
	}
	t.Log().Tracef("bypass export for unprovision on leader")
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if v, err := t.isUp(ctx); err != nil {
		return err
	} else if !v {
		t.Log().Infof("%s is already down", t.Label(ctx))
		return nil
	}
	if err := t.poolExport(); err != nil {
		return err
	}
	return nil
}

func (t *T) isUp(ctx context.Context) (bool, error) {
	pool := t.pool()
	if v, err := t.hasIt(ctx); err != nil {
		return false, err
	} else if !v {
		return false, nil
	}
	data, err := pool.Status(ctx, zfs.PoolStatusWithVerbose())
	if err != nil {
		return false, err
	}
	switch data.State {
	case "ONLINE":
		return true, nil
	case "SUSPENDED", "DEGRADED":
		t.StatusLog().Warn(strings.ToLower(data.State))
		return false, nil
	default:
		return false, nil
	}
}

func (t *T) Status(ctx context.Context) status.T {
	if v, err := t.isUp(ctx); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if v {
		return status.Up
	}
	return status.Down
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.Name
}

// poolImport imports the pool.
// 1/ try using a dev list cache, which is fastest
// 2/ fallback without dev list cache
//
// Parallel import can fail on Solaris 11.4, with a "no such
// pool available" error. Retry in this case, if we confirm the
// pool exists.
func (t *T) poolImport() error {
	var err error
	for i := 0; i < 10; i++ {
		err = t.poolImportTryDevice(false)
		if err == nil {
			return nil
		}
		time.Sleep(time.Second * 2)
	}
	return err
}

func (t *T) poolImportCacheFile() string {
	return filepath.Join(rawconfig.Paths.Var, "zpool.cache")
}

func (t *T) poolImportDeviceDir() string {
	return filepath.Join(t.VarDir(), "dev", "dsk")
}

func (t *T) poolImportTryDevice(quiet bool) error {
	if err := t.poolImportWithDevice(quiet); err == nil {
		return nil
	}
	return t.poolImportWithoutDevice(quiet)
}

func (t *T) poolImportWithoutDevice(quiet bool) error {
	c := t.poolImportCacheFile()
	fopts := []funcopt.O{
		zfs.PoolImportWithForce(),
		zfs.PoolImportWithOption("cachefile", c),
	}
	if quiet && t.Log().GetLevel() != zerolog.DebugLevel {
		fopts = append(fopts, zfs.PoolImportWithQuiet())
	}
	return t.pool().Import(fopts...)
}

func (t *T) poolImportWithDevice(quiet bool) error {
	d := t.poolImportDeviceDir()
	if !file.Exists(d) {
		return fmt.Errorf("%s does not exist", d)
	}
	c := t.poolImportCacheFile()
	fopts := []funcopt.O{
		zfs.PoolImportWithForce(),
		zfs.PoolImportWithOption("cachefile", c),
		zfs.PoolImportWithDevice(d),
	}
	if quiet && t.Log().GetLevel() != zerolog.DebugLevel {
		fopts = append(fopts, zfs.PoolImportWithQuiet())
	}
	return t.pool().Import(fopts...)
}

func (t *T) poolExport() error {
	pool := t.pool()
	if err := pool.Export(); err == nil {
		return nil
	}
	return pool.Export(zfs.PoolExportWithForce())
}

func (t *T) poolCreate() error {
	a := args.New()
	a.Append(t.CreateOptions...)
	a.DropOptionAndAnyValue("-m")
	a.DropOptionAndMatchingValue("-o", "^cachefile=.*")
	a.DropOptionAndMatchingValue("-o", "^multihost=.*")
	a.Append("-m", "legacy")
	a.Append("-o", "cachefile="+t.poolImportCacheFile())
	if runtime.GOOS == "linux" && t.Multihost == "true" {
		a.Append("-o", "multihost=on")
		if err := t.genHostID(); err != nil {
			return err
		}
	}
	return t.pool().Create(
		zfs.PoolCreateWithVDevs(t.VDev),
		zfs.PoolCreateWithArgs(a.Get()),
	)
}

func (t *T) poolDestroy() error {
	return t.pool().Destroy(
		zfs.PoolDestroyWithForce(),
	)
}

func (t *T) pool() *zfs.Pool {
	return &zfs.Pool{
		Name: t.Name,
		Log:  t.Log(),
	}
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	return t.unprovision(ctx)
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	return t.provision(ctx)
}

func (t *T) provision(ctx context.Context) error {
	if v, err := t.hasIt(ctx); err != nil {
		return err
	} else if v {
		t.Log().Infof("%s is already provisioned", t.Name)
		return nil
	}
	return t.poolCreate()
}

func (t *T) unprovision(ctx context.Context) error {
	if v, err := t.hasIt(ctx); err != nil {
		return err
	} else if !v {
		if err := t.poolImportTryDevice(true); err != nil {
			t.Log().Tracef("try import before destroy: %s", err)
			return nil
		}
	}
	return t.poolDestroy()
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	if v, err := t.hasIt(ctx); err != nil {
		return provisioned.Undef, err
	} else {
		return provisioned.FromBool(v), nil
	}
}

func (t *T) ExposedDevices(ctx context.Context) device.L {
	if l, err := t.poolListZDevs(ctx); err == nil {
		return t.toDevices(l)
	} else {
		return device.L{}
	}
}

// CurrentSize implements resource.Sizer.
//
// It is what the pool holds, which is less than its vdevs do: zfs keeps room
// for its labels and its reserve.
func (t *T) CurrentSize(ctx context.Context) (int64, error) {
	usage, err := t.pool().Usage(ctx)
	if err != nil {
		return 0, err
	}
	return usage.Size, nil
}

// ResizePlan implements resource.Resizer.
//
// The device below is asked for the size wanted, and the pool ends up a little
// under it, because zfs keeps room for its labels and its reserve. That is the
// same bargain a filesystem makes with the device under it.
//
// What zfs keeps is not added to the request, even though it is a fixed
// fraction of the pool, near a twenty fifth of it: a 300mi device carried a
// 288mi pool and a 600mi device carries a 576mi one.
//
// Adding it would mean measuring the gap between the device and the pool, and
// that gap is the growth itself once the device has grown and the pool has
// not, which every pass would then ask for again. It is the trap a drbd
// resource, an md member and a luks header all set, and the pool is the one
// place where there is no fixed property to read instead. So the request is
// passed down as it came, and the pool lands a little under it.
func (t *T) ResizePlan(ctx context.Context, to int64) (int64, error) {
	if len(t.SubDevices(ctx)) == 0 {
		return 0, fmt.Errorf("zpool %s rests on no device this object knows", t.Name)
	}
	return to, nil
}

// Resize implements resource.Resizer.
//
// A pool is not given a size: it is what its vdevs hold. Growing one is
// telling it to look again at devices that have grown.
func (t *T) Resize(ctx context.Context, to int64) error {
	devs := t.SubDevices(ctx)
	if len(devs) == 0 {
		return fmt.Errorf("zpool %s rests on no device this object knows", t.Name)
	}
	pool := t.pool()
	for _, dev := range devs {
		if err := pool.ExpandDevice(ctx, dev.Path()); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) SubDevices(ctx context.Context) device.L {
	if l, errUpd := t.updateSubDevsFile(ctx); errUpd == nil && l != nil {
		return t.toDevices(l)
	} else if l, errLoad := t.loadSubDevsFile(); errLoad == nil {
		t.Log().Tracef("update sub devs cache: %s", errUpd)
		return t.toDevices(l)
	} else {
		t.Log().Tracef("load sub devs cache: %s", errLoad)
		return device.L{}
	}
}

func (t *T) ReservableDevices(ctx context.Context) device.L {
	return t.SubDevices(ctx)
}

func (t *T) toDevices(l []string) device.L {
	log := t.Log()
	devs := make(device.L, 0)
	for _, s := range l {
		dev := device.New(s, device.WithLogger(log))
		devs = append(devs, dev)
	}
	return devs
}
