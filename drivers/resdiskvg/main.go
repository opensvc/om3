//go:build linux

package resdiskvg

import (
	"context"
	"fmt"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/udevadm"
)

type (
	T struct {
		resdisk.T
		Path    path.T
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
		Devices() ([]*device.T, error)
		PVs() ([]*device.T, error)
		ActiveLVs() ([]*device.T, error)
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
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t T) Start(ctx context.Context) error {
	if err := t.startTag(ctx); err != nil {
		return err
	}
	if v, err := t.isUp(); err != nil {
		return err
	} else if v {
		t.Log().Info().Msgf("%s is already up", t.Label())
		return nil
	}
	if err := t.vg().Activate(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.vg().Deactivate()
	})
	return nil
}

func (t T) Info() map[string]string {
	m := make(map[string]string)
	m["name"] = t.VGName
	return m
}

func (t T) Stop(ctx context.Context) error {
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
	if err := t.vg().Deactivate(); err != nil {
		return err
	}
	if err := t.stopTag(); err != nil {
		return err
	}
	return nil
}

func (t T) exists() (bool, error) {
	return t.vg().Exists()
}

func (t T) isUp() (bool, error) {
	return t.hasTag()
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
	if v, err := t.isUp(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if v {
		return status.Up
	}
	return status.Down
}

func (t T) Label() string {
	return t.VGName
}

func (t T) ProvisionLeader(ctx context.Context) error {
	vg := t.vg()
	vgi, ok := vg.(VGDriverProvisioner)
	if !ok {
		return fmt.Errorf("vg %s %s driver does not implement provisioning", vg.FQN(), vg.DriverName())
	}
	exists, err := vg.Exists()
	if err != nil {
		return err
	}
	if exists {
		t.Log().Info().Msgf("%s is already provisioned", vg.FQN())
		return nil
	}
	if pvs, err := object.Realdevpaths(t.PVs, t.Path.Namespace); err != nil {
		return err
	} else {
		return vgi.Create(t.Size, pvs, t.Options)
	}
}

func (t T) UnprovisionLeader(ctx context.Context) error {
	vg := t.vg()
	exists, err := vg.Exists()
	if err != nil {
		return err
	}
	if !exists {
		t.Log().Info().Msgf("%s is already unprovisioned", vg.FQN())
		return nil
	}
	if vgi, ok := vg.(VGDriverWiper); ok {
		_ = vgi.Wipe()
	} else {
		t.Log().Info().Msgf("%s wipe skipped: not implementing by %s", vg.FQN(), vg.DriverName())
	}
	vgi, ok := vg.(VGDriverUnprovisioner)
	if !ok {
		return fmt.Errorf("vg %s %s driver does not implement unprovisioning", vg.FQN(), vg.DriverName())
	}
	args := []string{"-f"}
	return vgi.Remove(args)
}

func (t T) Provisioned() (provisioned.T, error) {
	v, err := t.exists()
	return provisioned.FromBool(v), err
}

func (t T) ExposedDevices() []*device.T {
	if l, err := t.vg().ActiveLVs(); err == nil {
		return l
	} else {
		return []*device.T{}
	}
}

func (t T) ClaimedDevices() []*device.T {
	return t.SubDevices()
}

func (t T) SubDevices() []*device.T {
	if l, err := t.vg().PVs(); err != nil {
		t.Log().Debug().Err(err).Msg("")
		return []*device.T{}
	} else {
		return l
	}
}

func (t T) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}
