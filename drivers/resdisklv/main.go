//go:build linux

package resdisklv

import (
	"context"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/sizeconv"
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
	// LVDriverResizer is implemented by the lv implementations that can
	// report and change their size. A resource whose implementation does not
	// is refused when a resize is planned, not when it is applied.
	LVDriverResizer interface {
		Size(context.Context) (int64, error)
		Resize(context.Context, int64) error
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

// PoolCharge implements resource.PoolCharger. A logical volume takes its
// size of the volume group it is carved from.
//
// A size that is a share of the group, "100%FREE" and the like, is not a
// number the group can be rationed by, and is answered as none.
func (t *T) PoolCharge() (string, int64) {
	size, err := sizeconv.FromSize(t.Size)
	if err != nil {
		return "", 0
	}
	return t.VGName, size
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
	if err := t.removeHolders(ctx); err != nil {
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

func (t *T) removeHolders(ctx context.Context) error {
	return t.exposedDevice().RemoveHolders(ctx)
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
	if t.Size == "" {
		return fmt.Errorf("a logical volume is created with a size, and none is configured")
	}
	if err := lvi.Create(ctx, t.Size, t.CreateOptions); err != nil {
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

// ResizeRestsOn implements resource.ResizeRestsOn.
//
// A logical volume takes its space from its volume group, which no device
// leads to: what a volume group exposes is the logical volumes themselves.
func (t *T) ResizeRestsOn(_ context.Context) string {
	return "vg/" + t.VGName
}

func (t *T) ClaimedDevices(ctx context.Context) device.L {
	return t.ExposedDevices(ctx)
}

func (t *T) ExposedDevices(ctx context.Context) device.L {
	return device.L{t.exposedDevice()}
}

func (t *T) SubDevices(ctx context.Context) device.L {
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

func (t *T) resizer() (LVDriverResizer, error) {
	lv, ok := t.lv().(LVDriverResizer)
	if !ok {
		return nil, fmt.Errorf("the %s implementation cannot be resized", t.lv().DriverName())
	}
	return lv, nil
}

// CurrentSize implements resource.Sizer.
func (t *T) CurrentSize(ctx context.Context) (int64, error) {
	lv, err := t.resizer()
	if err != nil {
		return 0, err
	}
	return lv.Size(ctx)
}

// ResizePlan implements resource.Resizer.
//
// A logical volume holds what it is given, so it asks the volume group below
// it for the same size.
func (t *T) ResizePlan(ctx context.Context, to int64) (int64, error) {
	if _, err := t.resizer(); err != nil {
		return 0, err
	}
	if err := t.refuseShareSize(); err != nil {
		return 0, err
	}
	return to, nil
}

// refuseShareSize stops a resize of a logical volume whose size is written as
// a share of its volume group.
//
// lvm2 computes that share, once, when the volume is created, and om never
// learns what it came out as. A resize writes the size it reached back into
// the keyword it grew, and writing a count of bytes over "100%FREE" would
// answer a question nobody asked: the configuration says take what is left,
// and what is left is not that number.
//
// It is refused here, while the chain is still being planned, so nothing
// below has been grown by the time the answer comes.
func (t *T) refuseShareSize() error {
	if !strings.Contains(t.Size, "%") {
		return nil
	}
	return fmt.Errorf("its size is %s, a share of the volume group that lvm2 computes, which a resize cannot grow: write it as a size, or as an expression over the capacity of the volume group resource, like $(50%% * {disk#vg.capacity})", t.Size)
}

// Resize implements resource.Resizer.
func (t *T) Resize(ctx context.Context, to int64) error {
	lv, err := t.resizer()
	if err != nil {
		return err
	}
	return lv.Resize(ctx, to)
}
