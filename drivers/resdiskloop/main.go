package resdiskloop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/df"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/loop"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	T struct {
		resdisk.T
		File string `json:"file"`
		Size string `json:"size"`
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) loop() *loop.T {
	l := loop.New(
		loop.WithLogger(t.Log()),
	)
	return l
}

func (t *T) isUp(lo *loop.T) (bool, error) {
	return lo.FileExists(t.File)
}

func (t *T) Start(ctx context.Context) error {
	lo := t.loop()
	isUp, err := t.isUp(lo)
	if err != nil {
		return err
	}
	if isUp {
		stat, err := os.Stat(t.File)
		if err != nil {
			return err
		}
		if stat.Size() == 0 {
			return fmt.Errorf("%s exists but is empty", t.File)
		}
		t.Log().Infof("%s is already setup", t.Label(ctx))
		return nil
	}
	if err := t.autoProvision(ctx); err != nil {
		return err
	}
	if err := lo.Add(t.File); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return lo.FileDelete(t.File)
	})
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	lo := t.loop()
	if v, err := t.isUp(lo); err != nil {
		return err
	} else if !v {
		t.Log().Infof("%s is already down", t.Label(ctx))
	} else if err := lo.FileDelete(t.File); err != nil {
		return err
	}
	if err := t.autoUnprovision(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	lo := t.loop()
	loInfo, err := lo.FileGet(t.File)
	if err != nil {
		t.StatusLog().Warn("%s", err)
		return status.Undef
	}
	if loInfo == nil {
		return status.Down
	}
	stat, err := os.Stat(t.File)
	if err != nil {
		t.StatusLog().Warn("backend file deleted")
		return status.Warn
	}
	if stat.Size() == 0 {
		t.StatusLog().Warn("file exists but is empty")
		return status.Warn
	}
	return status.Up
}

func (t *T) fileExists() (os.FileInfo, error) {
	info, err := os.Stat(t.File)
	switch {
	case os.IsNotExist(err):
		return nil, nil
	case file.IsNotDir(err):
		return nil, nil
	case err != nil:
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil
	}
	return info, nil
}

func (t *T) Provisioned() (provisioned.T, error) {
	stat, err := t.fileExists()
	if err != nil {
		return provisioned.Undef, err
	}
	if stat == nil {
		return provisioned.False, nil
	}
	return provisioned.True, nil
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.File
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "file", Value: t.File},
	}
	return m, nil
}

func (t *T) isVolatile() bool {
	return df.HasTypeMount("tmpfs", t.File)
}

// autoProvision provisions the loop on start if the backing file is
// hosted on a tmpfs
func (t *T) autoProvision(ctx context.Context) error {
	if !t.isVolatile() {
		return nil
	}
	stat, err := t.fileExists()
	if err != nil {
		return err
	}
	if stat != nil {
		if err := t.removeEmptyBackendFile(); err != nil {
			return err
		}
	}
	return t.provision(ctx)
}

// autoUnprovision unprovisions the loop on stop if the backing file is
// hosted on a tmpfs
func (t *T) autoUnprovision(ctx context.Context) error {
	if stat, err := t.fileExists(); err != nil {
		return err
	} else if stat == nil {
		return nil
	}
	if !t.isVolatile() {
		return nil
	}
	return t.unprovision(ctx)
}

func (t *T) removeEmptyBackendFile() error {
	lo := t.loop()
	if err := lo.Delete(t.File); err != nil {
		return err
	}
	t.Log().Infof("remove empty existing file %s", t.File)
	if err := os.Remove(t.File); err != nil {
		return err
	}
	return nil
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	if stat, err := t.fileExists(); err != nil {
		return err
	} else if stat != nil {
		if stat.Size() == 0 {
			if err := t.removeEmptyBackendFile(); err != nil {
				return err
			}
		}
	}
	return t.provision(ctx)
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	if stat, err := t.fileExists(); err != nil {
		return err
	} else if stat == nil {
		return nil
	}
	return t.unprovision(ctx)
}

func (t *T) provisionDir(ctx context.Context) error {
	dir := filepath.Dir(t.File)
	if v, err := file.ExistsAndDir(dir); err != nil {
		return err
	} else if v {
		return nil
	}
	t.Log().Infof("create dir %s", dir)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		t.Log().Infof("unlink dir %s", dir)
		return os.Remove(dir)
	})
	return nil
}

func (t *T) provision(ctx context.Context) error {
	var (
		err  error
		f    *os.File
		size int64
	)
	if size, err = sizeconv.FromSize(t.Size); err != nil {
		return err
	}
	if err = t.provisionDir(ctx); err != nil {
		return err
	}
	t.Log().Infof("create file %s", t.File)
	if f, err = os.Create(t.File); err != nil {
		return err
	}
	defer f.Close()
	actionrollback.Register(ctx, func(ctx context.Context) error {
		t.Log().Infof("unlink file %s", t.File)
		return os.Remove(t.File)
	})
	offset := (size / 512 * 512) - 1
	t.Log().Infof("seek/write file, offset %d", offset)
	if _, err = f.Seek(offset, 0); err != nil {
		return err
	}
	if _, err = f.Write([]byte{0}); err != nil {
		return err
	}
	if err := t.setFileMode(); err != nil {
		return err
	}
	if err := t.setFileOwner(); err != nil {
		return err
	}
	return nil
}

func (t *T) unprovision(ctx context.Context) error {
	t.Log().Infof("unlink file %s", t.File)
	return os.RemoveAll(t.File)
}

func (t *T) exposedDevice(lo *loop.T) *device.T {
	i, err := lo.FileGet(t.File)
	if err != nil {
		return nil
	}
	if i == nil {
		return nil
	}
	dev := device.New(i.Name, device.WithLogger(t.Log()))
	return &dev
}

func (t *T) ExposedDevices() device.L {
	lo := t.loop()
	dev := t.exposedDevice(lo)
	if dev == nil {
		return device.L{}
	}
	return device.L{*dev}
}
