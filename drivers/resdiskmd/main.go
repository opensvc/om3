// +build linux

package resdiskmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/udevadm"
)

const (
	driverGroup = drivergroup.Disk
	driverName  = "md"
)

type (
	T struct {
		resdisk.T
		UUID   string   `json:"uuid"`
		Size   string   `json:"size"`
		Spares int      `json:"spares"`
		Chunk  *int64   `json:"chunk"`
		Layout string   `json:"layout"`
		Level  string   `json:"level"`
		Devs   []string `json:"devs"`
		Path   path.T   `json:"path"`
		Nodes  []string `json:"nodes"`
	}
	MDDriver interface {
		Activate() error
		Deactivate() error
		Resync() error
		IsActive() (bool, string, error)
		Exists() (bool, error)
		Devices() ([]*device.T, error)
		UUID() string
		IsAutoActivated() bool
		DisableAutoActivation() error
	}
	MDDriverProvisioner interface {
		Create(level string, devs []string, spares int, layout string, chunk *int64) error
		Remove() error
		Wipe() error
	}
)

func init() {
	resource.Register(driverGroup, driverName, New)
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
	m.AddKeyword(resdisk.BaseKeywords...)
	m.AddContext([]manifest.Context{
		{
			Key:  "path",
			Attr: "Path",
			Ref:  "object.path",
		},
		{
			Key:  "nodes",
			Attr: "Nodes",
			Ref:  "object.nodes",
		},
	}...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "uuid",
			Attr:     "UUID",
			Scopable: true,
			Text:     "The md uuid to use with mdadm assemble commands",
			Example:  "dev1",
		},
		{
			Option:       "devs",
			Attr:         "Devs",
			Scopable:     true,
			Converter:    converters.List,
			Provisioning: true,
			Text:         "The md member devices to use with mdadm create command",
			Example:      "/dev/mapper/23 /dev/mapper/24",
		},
		{
			Option:       "level",
			Attr:         "Level",
			Scopable:     true,
			Provisioning: true,
			Text:         "The md raid level to use with mdadm create command (see mdadm man for values)",
			Example:      "raid1",
		},
		{
			Option:       "chunk",
			Attr:         "Chunk",
			Scopable:     true,
			Converter:    converters.Size,
			Provisioning: true,
			Text:         "The md chunk size to use with mdadm create command. The value is adjusted to the first greater or equal multiple of 4.",
			Example:      "128k",
		},
		{
			Option:       "spares",
			Attr:         "Spares",
			Scopable:     true,
			Converter:    converters.Int,
			Provisioning: true,
			Text:         "The md number of spare devices to use with mdadm create command",
			Default:      "0",
			Example:      "1",
		},
	}...)
	return m
}

func (t T) Name() string {
	if t.Path.Namespace != "root" {
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

func (t T) Info() map[string]string {
	m := make(map[string]string)
	m["uuid"] = t.UUID
	return m
}

func (t T) Start(ctx context.Context) error {
	dev := t.md()
	_ = dev.DisableAutoActivation()
	if v, err := t.isUp(); err != nil {
		return err
	} else if v {
		t.Log().Info().Msgf("md %s is already assembled", t.Label())
		return nil
	}
	if err := dev.Activate(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return dev.Deactivate()
	})
	// drop the create_static_name(devpath) py code ??
	return nil
}

func (t T) Stop(ctx context.Context) error {
	dev := t.md()
	if v, err := t.isUp(); err != nil {
		return err
	} else if !v {
		t.Log().Info().Msgf("%s is already down", t.Label())
		return nil
	}
	if err := t.removeHolders(); err != nil {
		return err
	}
	udevadm.Settle()
	if err := dev.Deactivate(); err != nil {
		return err
	}
	return nil
}

func (t T) exists() (bool, error) {
	return t.md().Exists()
}

func (t T) isUp() (bool, error) {
	active, _, err := t.md().IsActive()
	return active, err
}

func (t T) removeHolders() error {
	for _, dev := range t.ExposedDevices() {
		if err := dev.RemoveHolders(); err != nil {
			return nil
		}
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	dev := t.md()
	v, msg, err := dev.IsActive()
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
		return status.Up
	}
	t.downStateAlerts()
	return status.Down
}

func (t T) Label() string {
	return t.UUID
}

func (t *T) ProvisionLeader(ctx context.Context) error {
	dev := t.md()
	devIntf, ok := dev.(MDDriverProvisioner)
	if !ok {
		return fmt.Errorf("md driver does not implement the provisioner interface")
	}
	exists, err := dev.Exists()
	if err != nil {
		return err
	}
	if exists {
		t.Log().Info().Msgf("md is already created")
		return nil
	}
	if err := devIntf.Create(t.Level, t.Devs, t.Spares, t.Layout, t.Chunk); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return devIntf.Remove()
	})
	t.Log().Info().Msgf("md uuid is %s", dev.UUID())
	if err := t.SetUUID(dev.UUID()); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.UnsetUUID()
	})
	return nil
}

func (t T) uuidKey() key.T {
	k := key.T{
		Section: t.RID(),
		Option:  "uuid",
	}
	if t.Shared {
		k.Section = k.Section + "@" + hostname.Hostname()
	}
	return k
}

func (t *T) SetUUID(uuid string) error {
	// set in this driver
	t.UUID = uuid

	// set in the object config file
	obj, err := object.NewConfigurerFromPath(t.Path)
	if err != nil {
		return err
	}
	op := keyop.T{
		Key:   t.uuidKey(),
		Op:    keyop.Set,
		Value: uuid,
	}
	if err = obj.SetKeys(op); err != nil {
		return err
	}
	return nil
}

func (t *T) UnsetUUID() error {
	// unset in the object config file
	obj, err := object.NewConfigurerFromPath(t.Path)
	if err != nil {
		return err
	}
	if err = obj.UnsetKeys(t.uuidKey()); err != nil {
		return err
	}

	// unset in this driver
	t.UUID = ""
	return nil
}

func (t *T) UnprovisionLeader(ctx context.Context) error {
	dev := t.md()
	exists, err := dev.Exists()
	if err != nil {
		return err
	}
	if !exists {
		t.Log().Info().Msgf("already unprovisioned")
		return nil
	}
	devIntf, ok := dev.(MDDriverProvisioner)
	if !ok {
		return fmt.Errorf("driver does not implement the provisioner interface")
	}
	if err := devIntf.Remove(); err != nil {
		return err
	}
	if err := t.UnsetUUID(); err != nil {
		return err
	}
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	v, err := t.exists()
	return provisioned.FromBool(v), err
}

func (t T) ExposedDevices() []*device.T {
	if t.UUID == "" {
		return []*device.T{}
	}
	if v, err := t.isUp(); err == nil && v {
		return []*device.T{device.New("/dev/md/"+t.Name(), device.WithLogger(t.Log()))}
	}
	return []*device.T{}
}

func (t T) SubDevices() []*device.T {
	if l, err := t.md().Devices(); err != nil {
		t.Log().Debug().Err(err).Msg("")
		return []*device.T{}
	} else {
		return l
	}
}

func (t T) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}

func (t T) PostSync() error {
	return t.md().DisableAutoActivation()
}

func (t T) PreSync() error {
	return t.dumpCacheFile()
}

func (t T) Resync(ctx context.Context) error {
	return t.md().Resync()
}

func (t T) ToSync() []string {
	if t.UUID == "" {
		return []string{}
	}
	if !t.IsShared() {
		return []string{}
	}
	return []string{t.cacheFile()}
}

func (t T) cacheFile() string {
	return filepath.Join(t.VarDir(), "disks")
}

func (t T) dumpCacheFile() error {
	p := t.cacheFile()
	dids := make([]string, 0)
	for _, dev := range t.SubDevices() {
		if did, err := dev.WWID(); did != "" && err != nil {
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

func (t T) loadCacheFile() ([]string, error) {
	p := t.cacheFile()
	data := make([]string, 0)
	b, err := file.ReadAll(p)
	if err != nil {
		return data, err
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return data, err
	}
	return data, nil
}

func (t T) downStateAlerts() error {
	if !t.IsShared() {
		return nil
	}
	dids, err := t.loadCacheFile()
	if err != nil {
		return err
	}
	t.Log().Debug().Msgf("loaded disk ids from cache: %s", dids)
	return nil
}
