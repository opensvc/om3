//go:build linux

package resdisklv

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/udevadm"
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
		Activate(context.Context) error
		Deactivate(context.Context) error
		IsActive(context.Context) (bool, error)
		Exists(context.Context) (bool, error)
		FQN() string
		Devices(context.Context) (device.L, error)
		DriverName() string
	}
	LVDriverProvisioner interface {
		Create(context.Context, string, []string) error
	}
	LVDriverUnprovisioner interface {
		Remove(context.Context, []string) error
	}
	LVDriverWiper interface {
		Wipe(context.Context) error
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) Start(ctx context.Context) error {
	if v, err := t.isUp(ctx); err != nil {
		return err
	} else if v {
		t.Log().Infof("%s is already up", t.Label(ctx))
		return nil
	}
	if err := t.lv().Activate(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.lv().Deactivate(ctx)
	})
	return nil
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "name", Value: t.LVName},
		{Key: "vg", Value: t.VGName},
	}
	return m, nil
}

func (t *T) Stop(ctx context.Context) error {
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
	return t.lv().Deactivate(ctx)
}

func (t *T) exists(ctx context.Context) (bool, error) {
	return t.lv().Exists(ctx)
}

func (t *T) isUp(ctx context.Context) (bool, error) {
	return t.lv().IsActive(ctx)
}

func (t *T) removeHolders() error {
	return t.exposedDevice().RemoveHolders()
}

func (t *T) fqn() string {
	return t.lv().FQN()
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
	return t.fqn()
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	lv := t.lv()
	lvi, ok := lv.(LVDriverProvisioner)
	if !ok {
		return fmt.Errorf("lv %s %s driver does not implement provisioning", lv.FQN(), lv.DriverName())
	}
	exists, err := lv.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		t.Log().Infof("%s is already provisioned", lv.FQN())
		return nil
	}
	if lvi.Create(ctx, t.Size, t.CreateOptions); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		if lvi, ok := lv.(LVDriverUnprovisioner); ok {
			return lvi.Remove(ctx, []string{"-f"})
		} else {
			return nil
		}
	})
	return nil
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	lv := t.lv()
	exists, err := lv.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		t.Log().Infof("%s is already unprovisioned", lv.FQN())
		return nil
	}
	if lvi, ok := lv.(LVDriverWiper); ok {
		_ = lvi.Wipe(ctx)
	} else {
		t.Log().Infof("%s wipe skipped: not implementing by %s", lv.FQN(), lv.DriverName())
	}
	lvi, ok := lv.(LVDriverUnprovisioner)
	if !ok {
		return fmt.Errorf("lv %s %s driver does not implement unprovisioning", lv.FQN(), lv.DriverName())
	}
	return lvi.Remove(ctx, []string{"-f"})
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	v, err := t.exists(ctx)
	return provisioned.FromBool(v), err
}

func (t *T) exposedDevice() device.T {
	return device.New(fmt.Sprintf("/dev/%s", t.fqn()), device.WithLogger(t.Log()))
}

func (t *T) ClaimedDevices() device.L {
	return t.ExposedDevices()
}

func (t *T) ExposedDevices() device.L {
	return device.L{t.exposedDevice()}
}

func (t *T) SubDevices() device.L {
	ctx := context.Background()
	if l, err := t.lv().Devices(ctx); err != nil {
		t.Log().Tracef("%s", err)
		return device.L{}
	} else {
		return l
	}
}

func (t *T) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}
