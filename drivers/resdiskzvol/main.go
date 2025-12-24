package resdiskzvol

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/zfs"
)

const (
	startedProp = "opensvc:started"
)

type (
	T struct {
		resdisk.T
		Name          string   `json:"name"`
		Size          *int64   `json:"size"`
		BlockSize     *int64   `json:"blocksize"`
		CreateOptions []string `json:"create_options"`
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) hasIt() (bool, error) {
	return t.zvol().Exists()
}

func (t *T) Stop(ctx context.Context) error {
	if v, err := t.isUp(); err != nil {
		return err
	} else if !v {
		t.Log().Infof("%s is already down", t.Label(ctx))
		return nil
	}
	if err := t.setStartedProp("false"); err != nil {
		return err
	}
	return nil
}

func (t *T) Start(ctx context.Context) error {
	if v, err := t.isUp(); err != nil {
		return err
	} else if v {
		t.Log().Infof("%s is already up", t.Label(ctx))
		return nil
	}
	if err := t.setStartedProp("true"); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.setStartedProp("false")
	})
	return nil
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "name", Value: t.Name},
		{Key: "pool", Value: zfs.DatasetName(t.Name).PoolName()},
		{Key: "device", Value: t.devpath()},
	}
	return m, nil
}

func (t *T) devpath() string {
	zn := zfs.DatasetName(t.Name)
	return fmt.Sprintf("/dev/%s/%s", zn.PoolName(), zn.BaseName())
}

func (t *T) zvol() *zfs.Vol {
	return &zfs.Vol{
		Name: t.Name,
		Log:  t.Log(),
	}
}

func (t *T) setStartedProp(s string) error {
	return t.zvol().SetProperty(startedProp, s)
}

func (t *T) getStartedProp() (string, error) {
	return t.zvol().GetProperty(startedProp)
}

func (t *T) isUp() (bool, error) {
	if v, err := t.hasIt(); err != nil {
		return false, err
	} else if !v {
		return false, nil
	}
	if v, err := t.getStartedProp(); err != nil {
		return false, err
	} else if v == "true" {
		return true, nil
	} else {
		return false, nil
	}
}

func (t *T) Status(ctx context.Context) status.T {
	if v, err := t.isUp(); err != nil {
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

func (t *T) zvolCreate() error {
	opts := make([]funcopt.O, 0)
	if t.Size != nil {
		opts = append(opts, zfs.VolCreateWithSize(uint64(*t.Size)))
	}
	if t.BlockSize != nil {
		opts = append(opts, zfs.VolCreateWithBlockSize(uint64(*t.BlockSize)))
	}
	return t.zvol().Create(opts...)
}

func (t *T) zvolDestroy() error {
	return t.zvol().Destroy(
		zfs.VolDestroyWithForce(),
	)
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	return t.unprovision(ctx)
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	return t.provision(ctx)
}

func (t *T) provision(ctx context.Context) error {
	if v, err := t.hasIt(); err != nil {
		return err
	} else if v {
		t.Log().Infof("%s is already provisioned", t.Name)
		return nil
	}
	return t.zvolCreate()
}

func (t *T) unprovision(ctx context.Context) error {
	if v, err := t.hasIt(); err != nil {
		return err
	} else if !v {
		t.Log().Infof("%s is already unprovisioned", t.Name)
		return nil
	}
	return t.zvolDestroy()
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	if v, err := t.hasIt(); err != nil {
		return provisioned.Undef, err
	} else {
		return provisioned.FromBool(v), nil
	}
}

func (t *T) ExposedDevices(ctx context.Context) device.L {
	p := t.devpath()
	return t.toDevices([]string{p})
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
