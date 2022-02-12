//go:build linux
// +build linux

package resdiskzvol

import (
	"context"
	"fmt"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/zfs"
)

const (
	driverGroup = drivergroup.Disk
	driverName  = "zvol"
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
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "name",
			Attr:     "Name",
			Required: true,
			Scopable: true,
			Text:     "The full name of the zfs volume in the ``<pool>/<name>`` form.",
			Example:  "tank/zvol1",
		},
		{
			Option:       "create_options",
			Attr:         "CreateOptions",
			Converter:    converters.Shlex,
			Scopable:     true,
			Provisioning: true,
			Text:         "The :cmd:`zfs create -V <name>` extra options.",
			Example:      "-o dedup=on",
		},
		{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Converter:    converters.Size,
			Provisioning: true,
			Text:         "The size of the zfs volume to create.",
			Example:      "10m",
		},
		{
			Option:       "blocksize",
			Attr:         "BlockSize",
			Scopable:     true,
			Converter:    converters.Size,
			Provisioning: true,
			Text:         "The blocksize of the zfs volume to create.",
			Example:      "256k",
		},
	}...)
	return m
}

func (t T) hasIt() (bool, error) {
	return t.zvol().Exists()
}

func (t T) Stop(ctx context.Context) error {
	if v, err := t.isUp(); err != nil {
		return err
	} else if !v {
		t.Log().Info().Msgf("%s is already down", t.Label())
		return nil
	}
	if err := t.setStartedProp("false"); err != nil {
		return err
	}
	return nil
}

func (t T) Start(ctx context.Context) error {
	if v, err := t.isUp(); err != nil {
		return err
	} else if v {
		t.Log().Info().Msgf("%s is already up", t.Label())
		return nil
	}
	if err := t.setStartedProp("true"); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.setStartedProp("false")
	})
	return nil
}

func (t T) Info() map[string]string {
	m := make(map[string]string)
	m["name"] = t.Name
	m["pool"] = zfs.ZfsName(t.Name).PoolName()
	m["device"] = t.devpath()
	return m
}

func (t T) devpath() string {
	zn := zfs.ZfsName(t.Name)
	return fmt.Sprintf("/dev/%s/%s", zn.PoolName(), zn.BaseName())
}

func (t T) zvol() *zfs.Vol {
	return &zfs.Vol{
		Name: t.Name,
		Log:  t.Log(),
	}
}

func (t T) setStartedProp(s string) error {
	return t.zvol().SetProperty(startedProp, s)
}

func (t T) getStartedProp() (string, error) {
	return t.zvol().GetProperty(startedProp)
}

func (t T) isUp() (bool, error) {
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

func (t T) Label() string {
	return t.Name
}

func (t T) zvolCreate() error {
	opts := make([]funcopt.O, 0)
	if t.Size != nil {
		opts = append(opts, zfs.VolCreateWithSize(uint64(*t.Size)))
	}
	if t.BlockSize != nil {
		opts = append(opts, zfs.VolCreateWithBlockSize(uint64(*t.BlockSize)))
	}
	return t.zvol().Create(opts...)
}

func (t T) zvolDestroy() error {
	return t.zvol().Destroy(
		zfs.VolDestroyWithForce(),
	)
}

func (t T) UnprovisionLeader(ctx context.Context) error {
	return t.unprovision(ctx)
}

func (t T) UnprovisionLeaded(ctx context.Context) error {
	return t.unprovision(ctx)
}

func (t T) ProvisionLeader(ctx context.Context) error {
	return t.provision(ctx)
}

func (t T) ProvisionLeaded(ctx context.Context) error {
	return t.provision(ctx)
}

func (t T) provision(ctx context.Context) error {
	if v, err := t.hasIt(); err != nil {
		return err
	} else if v {
		t.Log().Info().Msgf("%s is already provisioned", t.Name)
		return nil
	}
	return t.zvolCreate()
}

func (t T) unprovision(ctx context.Context) error {
	return t.zvolDestroy()
}

func (t T) Provisioned() (provisioned.T, error) {
	if v, err := t.hasIt(); err != nil {
		return provisioned.Undef, err
	} else {
		return provisioned.FromBool(v), nil
	}
}

func (t T) ExposedDevices() []*device.T {
	p := t.devpath()
	return t.toDevices([]string{p})
}

func (t T) toDevices(l []string) []*device.T {
	log := t.Log()
	devs := make([]*device.T, 0)
	for _, s := range l {
		dev := device.New(s, device.WithLogger(log))
		devs = append(devs, dev)
	}
	return devs
}
