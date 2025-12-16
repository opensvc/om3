//go:build linux

package resdiskvg

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/lvm2"
	"github.com/opensvc/om3/v3/util/udevadm"
)

type (
	T struct {
		resdisk.T
		Path    naming.Path
		VGName  string   `json:"vg"`
		Size    string   `json:"size"`
		Options []string `json:"options"`
		PVs     []string `json:"pvs"`
	}
	VGDriver interface {
		Activate(context.Context) error
		Deactivate(context.Context) error
		IsActive(context.Context) (bool, error)
		Exists(context.Context) (bool, error)
		FQN() string
		Devices(context.Context) (device.L, error)
		PVs(context.Context) (device.L, error)
		ActiveLVs() (device.L, error)
		DriverName() string
		AddTag(context.Context, string) error
		DelTag(context.Context, string) error
		HasTag(context.Context, string) (bool, error)
		Tags(context.Context) ([]string, error)
	}
	VGDriverProvisioner interface {
		Create(context.Context, string, []string, []string) error
	}
	VGDriverUnprovisioner interface {
		Remove(context.Context, []string) error
	}
	VGDriverWiper interface {
		Wipe(context.Context) error
	}
	VGDriverImportDeviceser interface {
		ImportDevices(context.Context) error
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) Start(ctx context.Context) error {
	if err := t.startTag(ctx); err != nil {
		return err
	}
	if v, err := t.isUp(ctx); err != nil {
		return err
	} else if v {
		t.Log().Infof("Volume group %s is already up", t.Label(ctx))
		return nil
	}
	if err := t.vg().Activate(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.vg().Deactivate(ctx)
	})
	return nil
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "name", Value: t.VGName},
	}
	return m, nil
}

func (t *T) Stop(ctx context.Context) error {
	if v, err := t.isUp(ctx); err != nil {
		return err
	} else if !v {
		t.Log().Infof("Volume group %s is already down", t.Label(ctx))
		return nil
	}
	if err := t.removeHolders(ctx); err != nil {
		return err
	}
	udevadm.Settle()
	if err := t.vg().Deactivate(ctx); err != nil {
		return err
	}
	if err := t.stopTag(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) exists(ctx context.Context) (bool, error) {
	return t.vg().Exists(ctx)
}

func (t *T) isUp(ctx context.Context) (bool, error) {
	return t.hasTag(ctx)
}

func (t *T) removeHolders(ctx context.Context) error {
	for _, dev := range t.ExposedDevices(ctx) {
		if err := dev.RemoveHolders(ctx); err != nil {
			return nil
		}
	}
	return nil
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
	return t.VGName
}

func (t *T) ProvisionAsFollower(ctx context.Context) error {
	if !t.IsShared() {
		return t.ProvisionAsLeader(ctx)
	}
	return lvm2.PVScan(t.Log())
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	vg := t.vg()
	vgi, ok := vg.(VGDriverProvisioner)
	if !ok {
		return fmt.Errorf("Volume group %s provisioning skipped: not implemented by driver %s", vg.FQN(), vg.DriverName())
	}
	exists, err := vg.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		t.Log().Infof("Volume group %s is already provisioned", vg.FQN())
		return nil
	}
	if pvs, err := vpath.HostDevpaths(ctx, t.PVs, t.Path.Namespace); err != nil {
		return err
	} else {
		return vgi.Create(ctx, t.Size, pvs, t.Options)
	}
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	vg := t.vg()
	exists, err := vg.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		t.Log().Infof("Volume group %s is already unprovisioned", vg.FQN())
		return nil
	}
	if vgi, ok := vg.(VGDriverWiper); ok {
		_ = vgi.Wipe(ctx)
	} else {
		t.Log().Infof("Volume group %s wipe skipped: not implemented by driver %s", vg.FQN(), vg.DriverName())
	}
	vgi, ok := vg.(VGDriverUnprovisioner)
	if !ok {
		return fmt.Errorf("vg %s %s driver does not implement unprovisioning", vg.FQN(), vg.DriverName())
	}
	args := []string{"-f"}
	return vgi.Remove(ctx, args)
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	v, err := t.exists(ctx)
	return provisioned.FromBool(v), err
}

func (t *T) ExposedDevices(ctx context.Context) device.L {
	if l, err := t.vg().ActiveLVs(); err == nil {
		return l
	} else {
		return device.L{}
	}
}

func (t *T) ClaimedDevices(ctx context.Context) device.L {
	return t.SubDevices(ctx)
}

func (t *T) ImportDevices(ctx context.Context) error {
	if vgi, ok := t.vg().(VGDriverImportDeviceser); ok {
		return vgi.ImportDevices(ctx)
	}
	return nil
}

func (t *T) ReservableDevices(ctx context.Context) device.L {
	return t.SubDevices(ctx)
}

func (t *T) SubDevices(ctx context.Context) device.L {
	if l, err := t.vg().PVs(ctx); err != nil {
		t.Log().Tracef("%s", err)
		return device.L{}
	} else {
		return l
	}
}

func (t *T) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}
