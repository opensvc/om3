package resdiskloop

import (
	"context"
	"os"
	"path/filepath"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/df"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/loop"
	"opensvc.com/opensvc/util/sizeconv"
)

const (
	driverGroup = drivergroup.Disk
	driverName  = "loop"
)

type (
	T struct {
		resdisk.T
		File string `json:"file"`
		Size string `json:"size"`
	}
)

func capabilitiesScanner() ([]string, error) {
	if !loop.IsCapable() {
		return []string{}, nil
	}
	return []string{"drivers.resource.disk.loop"}, nil
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
	m.AddKeyword(resdisk.BaseKeywords...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "file",
			Attr:     "File",
			Required: true,
			Scopable: true,
			Text:     "The loopback device backing file full path.",
			Example:  "/srv/{fqdn}-loop-{rindex}",
		},
		{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Text:         "The size of the loop file to provision.",
			Example:      "100m",
		},
	}...)
	return m
}

func init() {
	capabilities.Register(capabilitiesScanner)
	resource.Register(driverGroup, driverName, New)
}

func (t T) loop() *loop.T {
	l := loop.New(
		loop.WithLogger(t.Log()),
	)
	return l
}

func (t T) isUp(lo *loop.T) (bool, error) {
	return lo.FileExists(t.File)
}

func (t T) Start(ctx context.Context) error {
	lo := t.loop()
	if v, err := t.isUp(lo); err != nil {
		return err
	} else if v {
		t.Log().Info().Msgf("%s is already up", t.Label())
		return nil
	}
	if err := t.autoProvision(ctx); err != nil {
		return err
	}
	if err := lo.Add(t.File); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return lo.FileDelete(t.File)
	})
	return nil
}

func (t T) Stop(ctx context.Context) error {
	lo := t.loop()
	if v, err := t.isUp(lo); err != nil {
		return err
	} else if !v {
		t.Log().Info().Msgf("%s is already down", t.Label())
	} else if err := lo.FileDelete(t.File); err != nil {
		return err
	}
	if err := t.autoUnprovision(ctx); err != nil {
		return err
	}
	return nil
}

func (t T) Status(ctx context.Context) status.T {
	lo := t.loop()
	if v, err := t.isUp(lo); err != nil {
		t.StatusLog().Warn("%s", err)
		return status.Undef
	} else if v {
		return status.Up
	}
	return status.Down
}

func (t T) fileExists() bool {
	return file.ExistsAndRegular(t.File)
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.FromBool(t.fileExists()), nil
}

func (t T) Label() string {
	return t.File
}

func (t T) Info() map[string]string {
	m := make(map[string]string)
	m["file"] = t.File
	return m
}

func (t T) isVolatile() bool {
	return df.HasTypeMount("tmpfs", t.File)
}

//
// autoProvision provisions the loop on start if the backing file is
// hosted on a tmpfs
//
func (t T) autoProvision(ctx context.Context) error {
	if t.fileExists() {
		return nil
	}
	if !t.isVolatile() {
		return nil
	}
	return t.provision(ctx)
}

//
// autoUnprovision unprovisions the loop on stop if the backing file is
// hosted on a tmpfs
//
func (t T) autoUnprovision(ctx context.Context) error {
	if !t.fileExists() {
		return nil
	}
	if !t.isVolatile() {
		return nil
	}
	return t.unprovision(ctx)
}

func (t T) ProvisionLeader(ctx context.Context) error {
	if t.fileExists() {
		return nil
	}
	return t.provision(ctx)
}

func (t T) UnprovisionLeader(ctx context.Context) error {
	if !t.fileExists() {
		return nil
	}
	return t.unprovision(ctx)
}

func (t T) provisionDir(ctx context.Context) error {
	dir := filepath.Dir(t.File)
	if file.ExistsAndDir(dir) {
		return nil
	}
	t.Log().Info().Msgf("create dir %s", dir)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return os.Remove(dir)
	})
	return nil
}

func (t T) provision(ctx context.Context) error {
	var (
		err  error
		f    *os.File
		size int64
	)
	if err = t.provisionDir(ctx); err != nil {
		return err
	}
	t.Log().Info().Msgf("create file %s", t.File)
	if f, err = os.Create(t.File); err != nil {
		return err
	}
	defer f.Close()
	actionrollback.Register(ctx, func() error {
		return os.Remove(t.File)
	})
	if size, err = sizeconv.FromSize(t.Size); err != nil {
		return err
	}
	offset := (size / 512 * 512) - 1
	t.Log().Info().Msgf("seek/write file, offset %d", offset)
	if _, err = f.Seek(offset, 0); err != nil {
		return err
	}
	if _, err = f.Write([]byte{0}); err != nil {
		return err
	}
	return nil
}

func (t T) unprovision(ctx context.Context) error {
	t.Log().Info().Msgf("unlink %s", t.File)
	return os.RemoveAll(t.File)
}

func (t T) exposedDevice(lo *loop.T) *device.T {
	i, err := lo.FileGet(t.File)
	if err != nil {
		return nil
	}
	return device.New(i.Name, device.WithLogger(t.Log()))
}

func (t T) ExposedDevices() []*device.T {
	lo := t.loop()
	dev := t.exposedDevice(lo)
	if dev == nil {
		return make([]*device.T, 0)
	}
	return []*device.T{dev}
}
