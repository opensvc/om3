//go:build linux
// +build linux

package resdisklv

import (
	"context"
	"fmt"

	"opensvc.com/opensvc/core/actionrollback"
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
		LVName        string   `json:"name"`
		VGName        string   `json:"vg"`
		Size          string   `json:"size"`
		CreateOptions []string `json:"create_options"`
	}
	LVDriver interface {
		Activate() error
		Deactivate() error
		IsActive() (bool, error)
		Exists() (bool, error)
		FQN() string
		Devices() ([]*device.T, error)
		DriverName() string
	}
	LVDriverProvisioner interface {
		Create(string, []string) error
	}
	LVDriverUnprovisioner interface {
		Remove([]string) error
	}
	LVDriverWiper interface {
		Wipe() error
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t T) Start(ctx context.Context) error {
	if v, err := t.isUp(); err != nil {
		return err
	} else if v {
		t.Log().Info().Msgf("%s is already up", t.Label())
		return nil
	}
	if err := t.lv().Activate(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.lv().Deactivate()
	})
	return nil
}

func (t T) Info() map[string]string {
	m := make(map[string]string)
	m["name"] = t.LVName
	m["vg"] = t.VGName
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
	return t.lv().Deactivate()
}

func (t T) exists() (bool, error) {
	return t.lv().Exists()
}

func (t T) isUp() (bool, error) {
	return t.lv().IsActive()
}

func (t T) removeHolders() error {
	return t.exposedDevice().RemoveHolders()
}

func (t T) fqn() string {
	return t.lv().FQN()
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
	return t.fqn()
}

func (t T) ProvisionLeader(ctx context.Context) error {
	lv := t.lv()
	lvi, ok := lv.(LVDriverProvisioner)
	if !ok {
		return fmt.Errorf("lv %s %s driver does not implement provisioning", lv.FQN(), lv.DriverName())
	}
	exists, err := lv.Exists()
	if err != nil {
		return err
	}
	if exists {
		t.Log().Info().Msgf("%s is already provisioned", lv.FQN())
		return nil
	}
	return lvi.Create(t.Size, t.CreateOptions)
}

func (t T) UnprovisionLeader(ctx context.Context) error {
	lv := t.lv()
	exists, err := lv.Exists()
	if err != nil {
		return err
	}
	if !exists {
		t.Log().Info().Msgf("%s is already unprovisioned", lv.FQN())
		return nil
	}
	if lvi, ok := lv.(LVDriverWiper); ok {
		_ = lvi.Wipe()
	} else {
		t.Log().Info().Msgf("%s wipe skipped: not implementing by %s", lv.FQN(), lv.DriverName())
	}
	lvi, ok := lv.(LVDriverUnprovisioner)
	if !ok {
		return fmt.Errorf("lv %s %s driver does not implement unprovisioning", lv.FQN(), lv.DriverName())
	}
	args := []string{"-f"}
	return lvi.Remove(args)
}

func (t T) Provisioned() (provisioned.T, error) {
	v, err := t.exists()
	return provisioned.FromBool(v), err
}

func (t T) exposedDevice() *device.T {
	return device.New(fmt.Sprintf("/dev/%s", t.fqn()), device.WithLogger(t.Log()))
}

func (t T) ClaimedDevices() []*device.T {
	return t.ExposedDevices()
}

func (t T) ExposedDevices() []*device.T {
	return []*device.T{t.exposedDevice()}
}

func (t T) SubDevices() []*device.T {
	if l, err := t.lv().Devices(); err != nil {
		t.Log().Debug().Err(err).Msg("")
		return []*device.T{}
	} else {
		return l
	}
}

func (t T) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}
