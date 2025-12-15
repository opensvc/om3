//go:build linux

package resdiskmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/udevadm"
)

type (
	T struct {
		resdisk.T
		Bitmap string      `json:"bitmap"`
		Chunk  *int64      `json:"chunk"`
		Devs   []string    `json:"devs"`
		Layout string      `json:"layout"`
		Level  string      `json:"level"`
		Nodes  []string    `json:"nodes"`
		Path   naming.Path `json:"path"`
		Size   string      `json:"size"`
		Spares int         `json:"spares"`
		UUID   string      `json:"uuid"`
		Name   string      `json:"name"`
	}
	MDDriver interface {
		Activate(ctx context.Context) error
		Deactivate(ctx context.Context) error
		Resync(ctx context.Context) error
		IsActive(ctx context.Context) (bool, string, error)
		Exists(ctx context.Context) (bool, error)
		Devices(ctx context.Context) (device.L, error)
		UUID() string
		IsAutoActivated() bool
		DisableAutoActivation() error
	}
	MDDriverProvisioner interface {
		Create(ctx context.Context, level string, devs []string, spares int, layout string, chunk *int64, bitmap string) error
		Remove(ctx context.Context) error
		Wipe(ctx context.Context) error
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) GetName() string {
	if t.Name != "" {
		return t.Name
	} else if t.Path.Namespace != naming.NsRoot {
		return fmt.Sprintf(
			"%s.%s.%s",
			strings.ToLower(t.Path.Namespace),
			strings.Split(t.Path.Name, ".")[0],
			strings.ReplaceAll(t.RID(), "#", "."),
		)
	} else {
		return fmt.Sprintf(
			"%s.%s",
			strings.Split(t.Path.Name, ".")[0],
			strings.ReplaceAll(t.RID(), "#", "."),
		)
	}
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "name", Value: t.GetName()},
		{Key: "uuid", Value: t.UUID},
	}
	return m, nil
}

func (t *T) Start(ctx context.Context) error {
	dev := t.md()
	_ = dev.DisableAutoActivation()
	if v, err := t.isUp(ctx); err != nil {
		return err
	} else if v {
		t.Log().Infof("md %s is already assembled", t.Label(ctx))
		return nil
	}
	if err := dev.Activate(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return dev.Deactivate(ctx)
	})
	// drop the create_static_name(devpath) py code ??
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	dev := t.md()
	if v, err := t.isUp(ctx); err != nil {
		return err
	} else if !v {
		t.Log().Infof("%s is already down", t.Label(ctx))
		return nil
	}
	if err := t.removeHolders(); err != nil {
		return err
	}
	udevadm.Settle()
	if err := dev.Deactivate(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) exists(ctx context.Context) (bool, error) {
	return t.md().Exists(ctx)
}

func (t *T) isUp(ctx context.Context) (bool, error) {
	active, _, err := t.md().IsActive(ctx)
	return active, err
}

func (t *T) removeHolders() error {
	for _, dev := range t.ExposedDevices() {
		if err := dev.RemoveHolders(); err != nil {
			return nil
		}
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	dev := t.md()
	v, msg, err := dev.IsActive(ctx)
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	if msg != "" {
		t.StatusLog().Warn(msg)
	}
	if dev.IsAutoActivated() {
		t.StatusLog().Warn("auto-assemble is not disabled")
	}
	if v {
		if err := t.dumpCacheFile(); err != nil {
			t.StatusLog().Warn("dump disks cache: %s", err)
		}
		return status.Up
	}
	t.downStateAlerts()
	return status.Down
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.UUID
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	dev := t.md()
	devIntf, ok := dev.(MDDriverProvisioner)
	if !ok {
		return fmt.Errorf("md driver does not implement the provisioner interface")
	}
	exists, err := dev.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		t.Log().Infof("md is already created")
		return nil
	}
	if err := devIntf.Create(ctx, t.Level, t.Devs, t.Spares, t.Layout, t.Chunk, t.Bitmap); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return devIntf.Remove(ctx)
	})
	t.Log().Infof("md uuid is %s", dev.UUID())
	if err := t.SetUUID(ctx, dev.UUID()); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.UnsetUUID(ctx)
	})
	return nil
}

func (t *T) uuidKey() key.T {
	k := key.T{
		Section: t.RID(),
		Option:  "uuid",
	}
	if !t.Shared {
		k.Option = k.Option + "@" + hostname.Hostname()
	}
	return k
}

func (t *T) SetUUID(ctx context.Context, uuid string) error {
	// set in this driver
	t.UUID = uuid

	// set in the object config file
	obj, err := object.NewConfigurer(t.Path)
	if err != nil {
		return err
	}
	op := keyop.T{
		Key:   t.uuidKey(),
		Op:    keyop.Set,
		Value: uuid,
	}
	if err = obj.Set(ctx, op); err != nil {
		return err
	}
	return nil
}

func (t *T) UnsetUUID(ctx context.Context) error {
	// unset in the object config file
	obj, err := object.NewConfigurer(t.Path)
	if err != nil {
		return err
	}
	if err = obj.Unset(ctx, t.uuidKey()); err != nil {
		return err
	}

	// unset in this driver
	t.UUID = ""
	return nil
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	dev := t.md()
	exists, err := dev.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		t.Log().Infof("already unprovisioned")
		return nil
	}
	devIntf, ok := dev.(MDDriverProvisioner)
	if !ok {
		return fmt.Errorf("driver does not implement the provisioner interface")
	}
	if err := devIntf.Wipe(ctx); err != nil {
		return err
	}
	if err := t.UnsetUUID(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	v, err := t.exists(ctx)
	return provisioned.FromBool(v), err
}

func (t *T) ExposedDevices() device.L {
	ctx := context.Background()
	if t.UUID == "" {
		return device.L{}
	}
	if v, err := t.isUp(ctx); err == nil && v {
		return device.L{device.New("/dev/md/"+t.GetName(), device.WithLogger(t.Log()))}
	}
	return device.L{}
}

func (t *T) SubDevices() device.L {
	ctx := context.Background()
	if l, err := t.md().Devices(ctx); err != nil {
		t.Log().Tracef("%s", err)
		return device.L{}
	} else {
		return l
	}
}

func (t *T) ReservableDevices() device.L {
	return t.SubDevices()
}

func (t *T) ClaimedDevices() device.L {
	return t.SubDevices()
}

func (t *T) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}

func (t *T) Ingest(ctx context.Context) error {
	return t.md().DisableAutoActivation()
}

func (t *T) Resync(ctx context.Context) error {
	return t.md().Resync(ctx)
}

func (t *T) ToSync(ctx context.Context) []string {
	if t.UUID == "" {
		return []string{}
	}
	if !t.IsShared() {
		return []string{}
	}
	return []string{t.cacheFile()}
}

func (t *T) cacheFile() string {
	return filepath.Join(t.VarDir(), "disks")
}

func (t *T) dumpCacheFile() error {
	p := t.cacheFile()
	dids := make([]string, 0)
	for _, dev := range t.SubDevices() {
		if did, err := dev.WWID(); did != "" && err == nil {
			dids = append(dids, did)
		}
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(dids)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	return nil
}

func (t *T) loadCacheFile() ([]string, error) {
	p := t.cacheFile()
	data := make([]string, 0)
	b, err := os.ReadFile(p)
	if err != nil {
		return data, err
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return data, err
	}
	return data, nil
}

func (t *T) downStateAlerts() error {
	if !t.IsShared() {
		return nil
	}
	dids, err := t.loadCacheFile()
	if err != nil {
		return err
	}
	var notFound []string
	for _, did := range dids {
		_, err = os.Stat("/dev/disk/by-id/scsi-" + did)
		if errors.Is(err, os.ErrNotExist) {
			notFound = append(notFound, did)
		}
	}
	if notFound != nil {
		slices.Sort(notFound)
		t.StatusLog().Warn("md members missing: %s", strings.Join(notFound, ","))
	}
	t.Log().Tracef("loaded disk ids from cache: %s", dids)
	return nil
}
