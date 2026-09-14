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
	"github.com/opensvc/om3/v3/util/sizeconv"
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
		ActiveLVDevices() (device.L, error)
		DriverName() string
		AddTag(context.Context, string) error
		DelTag(context.Context, string) error
		HasTag(context.Context, string) (bool, error)
		Tags(context.Context) ([]string, error)
		GetLVSummary(context.Context) (lvm2.LVSummary, error)
		NeedActivate(lvm2.LVSummary) bool
	}
	VGDriverResizer interface {
		Size(context.Context) (int64, error)
		ExtentSize(context.Context) (int64, error)
		ResizePV(context.Context, string, int64) error
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
	vg := t.vg()
	exists, err := vg.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		if err := lvm2.PVScan(t.Log()); err != nil {
			return err
		}
	}
	if err := t.startTag(ctx); err != nil {
		return err
	}
	if v, err := t.hasTag(ctx); err != nil {
		return err
	} else if v {
		if r, err := vg.GetLVSummary(ctx); err != nil {
			// log the unexpected error, but we can continue (Activate will be called)
			t.Log().Warnf("can't detect if volume group has activable volumes: %s", err)
		} else if vg.NeedActivate(r) {
			t.Log().Debugf("Volume group %s need activation: has %d of %d volumes activated", t.Label(ctx), r.Activated, r.Total)
		} else {
			t.Log().Infof("Volume group %s is already up", t.Label(ctx))
			return nil
		}
	}
	if err := t.vg().Activate(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.vg().Deactivate(ctx)
	})
	return nil
}

// resizer returns the volume group driver when it can report and change a
// size, and says which driver cannot when it does not.
func (t *T) resizer() (VGDriverResizer, error) {
	vg := t.vg()
	i, ok := vg.(VGDriverResizer)
	if !ok {
		return nil, fmt.Errorf("%s volume groups cannot be resized", vg.DriverName())
	}
	return i, nil
}

// CurrentSize implements resource.Sizer.
func (t *T) CurrentSize(ctx context.Context) (int64, error) {
	vg, err := t.resizer()
	if err != nil {
		return 0, err
	}
	return vg.Size(ctx)
}

// ResizePlan implements resource.Resizer.
//
// A volume group offers its logical volumes less than its physical volumes
// hold, by the metadata lvm keeps and by what does not fill a whole extent.
// What it asks of the device below is the size wanted plus what it is short
// today, which over-asks by at most one extent and never under-asks.
func (t *T) ResizePlan(ctx context.Context, to int64) (int64, error) {
	vg, err := t.resizer()
	if err != nil {
		return 0, err
	}
	pvs, err := t.vg().PVs(ctx)
	if err != nil {
		return 0, err
	}
	if len(pvs) != 1 {
		return 0, fmt.Errorf("volume group %s has %d physical volumes: which of them to resize is not something this can decide",
			t.VGName, len(pvs))
	}
	pvSize, err := pvs[0].Size()
	if err != nil {
		return 0, err
	}
	vgSize, err := vg.Size(ctx)
	if err != nil {
		return 0, err
	}
	overhead := pvSize - vgSize
	if overhead < 0 {
		overhead = 0
	}

	// A volume group hands out whole extents, so a logical volume asking for
	// a size that is not one gets the next one up. Ask for that here, or the
	// group ends up one extent short of what the volume above it needs.
	extent, err := vg.ExtentSize(ctx)
	if err != nil {
		return 0, err
	}
	to = sizeconv.RoundUp(to, extent)

	// A block device holds whole sectors, so that is what it asks for.
	return sizeconv.RoundUp(to+overhead, 512), nil
}

// Resize implements resource.Resizer.
//
// Growing takes the whole device, which the plan made sure is large enough.
// Shrinking sets the physical volume size explicitly, which lvm refuses if
// extents beyond it are in use.
func (t *T) Resize(ctx context.Context, to int64) error {
	vg, err := t.resizer()
	if err != nil {
		return err
	}
	pvs, err := t.vg().PVs(ctx)
	if err != nil {
		return err
	}
	if len(pvs) != 1 {
		return fmt.Errorf("volume group %s has %d physical volumes: which of them to resize is not something this can decide",
			t.VGName, len(pvs))
	}
	from, err := vg.Size(ctx)
	if err != nil {
		return err
	}
	size := int64(0)
	if to < from {
		// Give space back: the physical volume keeps what the group needs
		// plus what lvm keeps for itself, which is the same arithmetic the
		// plan did.
		if size, err = t.ResizePlan(ctx, to); err != nil {
			return err
		}
	}
	return vg.ResizePV(ctx, pvs[0].Path(), size)
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

// isUp checks if the volume group is active by verifying the presence of a specific tag.
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
		r, err := t.vg().GetLVSummary(ctx)
		if err != nil {
			t.StatusLog().Error("%s", err)
			return status.Undef
		}
		if r.Activated != r.Total {
			t.StatusLog().Warn("%d of %d volumes are not activated", r.Total-r.Activated, r.Total)
		}
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
	if l, err := t.vg().ActiveLVDevices(); err == nil {
		return l
	} else {
		return device.L{}
	}
}

// ResizeProvides implements resource.ResizeProvides, so the logical volumes
// of this group find it: no device leads from one to the other.
func (t *T) ResizeProvides(_ context.Context) string {
	return "vg/" + t.VGName
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
