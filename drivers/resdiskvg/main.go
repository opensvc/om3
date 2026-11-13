//go:build linux

package resdiskvg

import (
	"context"
	"fmt"
	"strings"

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
		Free(context.Context) (int64, error)
		ExtentSize(context.Context) (int64, error)
		ResizePV(context.Context, string, int64) error
	}

	// VGDriverPEStarter is implemented by a volume group driver that says the
	// offset of the first extent of its physical volume, the bytes it keeps
	// for its metadata whatever the device holds.
	VGDriverPEStarter interface {
		PEStart(context.Context) (int64, error)
	}
	VGDriverProvisioner interface {
		Create(context.Context, string, []string, []string) error
	}
	VGDriverUnprovisioner interface {
		Remove(context.Context, []string) error
	}
	VGDriverPVRemover interface {
		RemovePVs(context.Context, device.L) error
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

// CurrentFree implements resource.Freer.
func (t *T) CurrentFree(ctx context.Context) (int64, error) {
	vg, err := t.resizer()
	if err != nil {
		return 0, err
	}
	return vg.Free(ctx)
}

// ResizePlan implements resource.Resizer.
//
// A volume group offers its logical volumes less than its physical volumes
// hold, by the metadata lvm keeps and by what does not fill a whole extent.
// What it asks of the device below is the size wanted plus what it keeps for
// itself today, which over-asks by at most one extent and never under-asks.
func (t *T) ResizePlan(ctx context.Context, to int64) (int64, error) {
	overhead, extent, err := t.resizeUnits(ctx)
	if err != nil {
		return 0, err
	}
	return vgAsks(to, overhead, extent), nil
}

// ResizeHandsOut implements resource.ResizeHandsOuter.
//
// A volume group given a physical volume of below hands out below less what
// it keeps for itself, in whole extents. It is the inverse of ResizePlan, so
// what it answers is no more than what lvm hands out: the cut it measures
// holds what did not fill an extent at the size the physical volume has now,
// and that part may fill one at another size.
func (t *T) ResizeHandsOut(ctx context.Context) (func(below int64) int64, error) {
	overhead, extent, err := t.resizeUnits(ctx)
	if err != nil {
		return nil, err
	}
	return func(below int64) int64 { return vgHandsOut(below, overhead, extent) }, nil
}

// resizeUnits returns what the group keeps of its physical volume for itself,
// and the extent it hands out whole.
//
// What it keeps is the offset of its first extent, not the gap between the
// device and the group: once the device has grown and the group has not been
// grown onto it, that gap is the growth itself, and counting it as metadata
// asks for it a second time. What is left at the end of the device, less than
// an extent, is no part of either: the extents are handed out whole. The gap
// to the device stands in for a driver that cannot say.
func (t *T) resizeUnits(ctx context.Context) (overhead, extent int64, err error) {
	vg, err := t.resizer()
	if err != nil {
		return 0, 0, err
	}
	pvs, err := t.vg().PVs(ctx)
	if err != nil {
		return 0, 0, err
	}
	if len(pvs) != 1 {
		return 0, 0, fmt.Errorf("volume group %s has %d physical volumes: which of them to resize is not something this can decide",
			t.VGName, len(pvs))
	}
	if extent, err = vg.ExtentSize(ctx); err != nil {
		return 0, 0, err
	}
	if i, ok := vg.(VGDriverPEStarter); ok {
		if peStart, err := i.PEStart(ctx); err == nil && peStart > 0 {
			return peStart, extent, nil
		}
	}
	pvSize, err := pvs[0].Size()
	if err != nil {
		return 0, 0, err
	}
	vgSize, err := vg.Size(ctx)
	if err != nil {
		return 0, 0, err
	}
	overhead = pvSize - vgSize
	if overhead < 0 {
		overhead = 0
	}
	return overhead, extent, nil
}

// vgAsks is the size a physical volume has to hold for a group keeping
// overhead of it to hand out to.
//
// A group hands out whole extents, so a logical volume asking for a size that
// is not one gets the next one up: that is what is asked, or the group ends up
// one extent short of what the volume above it needs. A block device holds
// whole sectors, so that is what it asks for.
func vgAsks(to, overhead, extent int64) int64 {
	return sizeconv.RoundUp(sizeconv.RoundUp(to, extent)+overhead, 512)
}

// vgHandsOut is what a group keeping overhead of a physical volume of below
// hands out: the rest, in whole extents.
func vgHandsOut(below, overhead, extent int64) int64 {
	n := sizeconv.RoundDown(below-overhead, extent)
	if n < 0 {
		return 0
	}
	return n
}

// ResizeSpansBelow implements resource.ResizeSpansBelow.
//
// A volume group takes up the physical volume it is created on, keeping a cut
// of it for its own metadata and for what does not fill a whole extent.
func (t *T) ResizeSpansBelow() bool {
	return true
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

// existsFresh says the volume group exists, a vg found checked again once
// the lvm cache refreshed: with lvmetad, the cache of a node keeps listing
// a vg of a shared storage another node removed, as the leader of its
// unprovision, after the followers, and the next provision would refuse it,
// or adopt a vg gone.
func (t *T) existsFresh(ctx context.Context) (bool, error) {
	exists, err := t.vg().Exists(ctx)
	if err != nil || !exists {
		return exists, err
	}
	if err := lvm2.PVScan(t.Log()); err != nil {
		t.Log().Debugf("refresh the lvm cache: %s", err)
		return exists, nil
	}
	return t.vg().Exists(ctx)
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
	exists, err := t.existsFresh(ctx)
	if err != nil {
		return err
	}
	if exists {
		t.Log().Infof("Volume group %s is already provisioned", vg.FQN())
		return nil
	}
	if pvs, err := vpath.DevpathsOf(ctx, t.RID(), t.PVs, t.Path.Namespace, vpath.ResolverOf(t.GetObject())); err != nil {
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
	// The physical volumes are read before the group goes, which is what
	// lists them.
	pvs, err := vg.PVs(ctx)
	if err != nil {
		return fmt.Errorf("list the physical volumes of %s: %w", vg.FQN(), err)
	}
	args := []string{"-f"}
	if err := vgi.Remove(ctx, args); err != nil {
		return err
	}
	return t.removePVs(ctx, vg, pvs)
}

// removePVs removes the lvm label of the physical volumes pvs of the group
// just removed, so the unprovision leaves the devices as the provision found
// them: its vgcreate labeled them. A physical volume lvm misses is listed as
// "[unknown]", with no device to unlabel.
func (t *T) removePVs(ctx context.Context, vg VGDriver, pvs device.L) error {
	vgi, ok := vg.(VGDriverPVRemover)
	if !ok {
		t.Log().Infof("Volume group %s physical volumes removal skipped: not implemented by driver %s", vg.FQN(), vg.DriverName())
		return nil
	}
	l := make(device.L, 0, len(pvs))
	for _, pv := range pvs {
		if strings.HasPrefix(pv.Path(), "/") {
			l = append(l, pv)
		}
	}
	return vgi.RemovePVs(ctx, l)
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

// VolumeGroupName is the name of the volume group, by which a check of its
// usage is attributed to the object.
func (t *T) VolumeGroupName() string {
	return t.VGName
}
