//go:build linux

package resdiskvg

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/vpath"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/udevadm"
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
		Activate() error
		Deactivate() error
		IsActive() (bool, error)
		Exists() (bool, error)
		FQN() string
		Devices() (device.L, error)
		PVs() (device.L, error)
		ActiveLVs() (device.L, error)
		DriverName() string
		AddTag(string) error
		DelTag(string) error
		HasTag(string) (bool, error)
		Tags() ([]string, error)
	}
	VGDriverProvisioner interface {
		Create(string, []string, []string) error
	}
	VGDriverUnprovisioner interface {
		Remove([]string) error
	}
	VGDriverWiper interface {
		Wipe() error
	}
	VGDriverImportDeviceser interface {
		ImportDevices() error
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
	if v, err := t.isUp(); err != nil {
		return err
	} else if v {
		t.Log().Infof("Volume group %s is already up", t.Label(ctx))
		return nil
	}
	if err := t.vg().Activate(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.vg().Deactivate()
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
	if v, err := t.isUp(); err != nil {
		return err
	} else if !v {
		t.Log().Infof("Volume group %s is already down", t.Label(ctx))
		return nil
	}
	if err := t.removeHolders(); err != nil {
		return err
	}
	udevadm.Settle()
	if err := t.vg().Deactivate(); err != nil {
		return err
	}
	if err := t.stopTag(); err != nil {
		return err
	}
	return nil
}

func (t *T) exists() (bool, error) {
	return t.vg().Exists()
}

func (t *T) isUp() (bool, error) {
	return t.hasTag()
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
	return t.VGName
}

func (t *T) ProvisionLeader(ctx context.Context) error {
	vg := t.vg()
	vgi, ok := vg.(VGDriverProvisioner)
	if !ok {
		return fmt.Errorf("Volume group %s provisioning skipped: not implemented by driver %s", vg.FQN(), vg.DriverName())
	}
	exists, err := vg.Exists()
	if err != nil {
		return err
	}
	if exists {
		t.Log().Infof("Volume group %s is already provisioned", vg.FQN())
		return nil
	}
	if pvs, err := vpath.HostDevpaths(t.PVs, t.Path.Namespace); err != nil {
		return err
	} else {
		return vgi.Create(t.Size, pvs, t.Options)
	}
}

func (t *T) UnprovisionLeader(ctx context.Context) error {
	vg := t.vg()
	exists, err := vg.Exists()
	if err != nil {
		return err
	}
	if !exists {
		t.Log().Infof("Volume group %s is already unprovisioned", vg.FQN())
		return nil
	}
	if vgi, ok := vg.(VGDriverWiper); ok {
		_ = vgi.Wipe()
	} else {
		t.Log().Infof("Volume group %s wipe skipped: not implemented by driver %s", vg.FQN(), vg.DriverName())
	}
	vgi, ok := vg.(VGDriverUnprovisioner)
	if !ok {
		return fmt.Errorf("vg %s %s driver does not implement unprovisioning", vg.FQN(), vg.DriverName())
	}
	args := []string{"-f"}
	return vgi.Remove(args)
}

func (t *T) Provisioned() (provisioned.T, error) {
	v, err := t.exists()
	return provisioned.FromBool(v), err
}

func (t *T) ExposedDevices() device.L {
	if l, err := t.vg().ActiveLVs(); err == nil {
		return l
	} else {
		return device.L{}
	}
}

func (t *T) ClaimedDevices() device.L {
	return t.SubDevices()
}

func (t *T) ImportDevices() error {
	if vgi, ok := t.vg().(VGDriverImportDeviceser); ok {
		return vgi.ImportDevices()
	}
	return nil
}

func (t *T) ReservableDevices() device.L {
	return t.SubDevices()
}

func (t *T) SubDevices() device.L {
	if l, err := t.vg().PVs(); err != nil {
		t.Log().Debugf("%s", err)
		return device.L{}
	} else {
		return l
	}
}

func (t *T) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}
