/*
Package resvol is the volume resource driver

A volume resource is linked to a volume object named <name> in the
namespace of the service.

The volume object contains disk and fs resources configured by the
<pool> that created it, so the service doesn't have to embed
driver keywords that would prevent the service from being run on
another cluster with different capabilities.

Access:
* rwo  Read Write Once
* rwx  Read Write Many
* roo  Read Only Once
* rox  Read Only Many
*/
package resvol

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/core/volaccess"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	T struct {
		resource.T
		datarecv.DataRecv
		Name     string `json:"name"`
		Access   string `json:"access"`
		Pool     string `json:"pool"`
		PoolType string `json:"type"`
		Size     *int64 `json:"size"`
		Format   bool   `json:"format"`
		VolNodes []string

		// Context
		Path          naming.Path
		Topology      topology.T
		Nodes         []string
		ObjectParents []string
	}
)

const (
	Usage int = iota
	NoUsage
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) startVolume(ctx context.Context, volume object.Vol) error {
	if volumeStatus, err := t.statusVolume(ctx, volume); err != nil {
		return err
	} else if volumeStatus.Avail.Is(status.Up, status.StandbyUpWithUp) {
		t.Log().Infof("volume %s is already up", volume.Path())
		return nil
	}
	if err := volume.Start(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.stopVolume(ctx, volume, false)
	})
	return nil
}

func (t *T) stopVolume(ctx context.Context, volume object.Vol, force bool) error {
	ctx = actioncontext.WithForce(ctx, force)
	holders, err := volume.HoldersExcept(ctx, t.Path)
	if err != nil {
		return err
	}
	if len(holders) > 0 {
		t.Log().Infof("skip volume %s stop: active users: %s", volume.Path(), holders)
		return nil
	}
	return volume.Stop(ctx)
}

func (t *T) statusVolume(ctx context.Context, volume object.Vol) (instance.Status, error) {
	return volume.FreshStatus(ctx)
}

func (t *T) Start(ctx context.Context) error {
	volume, err := t.Volume()
	if err != nil {
		t.Log().Errorf("%s", err)
		return fmt.Errorf("volume %s does not exist (and no pool can create it)", t.name())
	}
	if !volume.Path().Exists() {
		return fmt.Errorf("volume %s does not exist", t.name())
	}
	if err = t.startVolume(ctx, volume); err != nil {
		return err
	}
	if err = t.startFlag(ctx); err != nil {
		return err
	}
	if err = t.DataRecv.Do(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) stopFlag(ctx context.Context) error {
	if !t.flagInstalled() {
		return nil
	}
	if err := t.uninstallFlag(); err != nil {
		return err
	}
	return nil
}

func (t *T) startFlag(ctx context.Context) error {
	if t.flagInstalled() {
		return nil
	}
	if err := t.installFlag(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.uninstallFlag()
	})
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if err := t.stopFlag(ctx); err != nil {
		return err
	}
	volume, err := t.Volume()
	if err != nil {
		return err
	}
	if err = t.stopVolume(ctx, volume, false); err != nil {
		return err
	}
	return nil
}

func (t *T) name() string {
	if t.Name != "" {
		return t.Name
	}
	return t.Path.Name + "-vol-" + t.ResourceID.Index()
}

func (t *T) CanInstall(ctx context.Context) (bool, error) {
	volume, err := t.Volume()
	if err != nil {
		return false, err
	}
	st, err := volume.Status(ctx)
	if err != nil {
		return false, err
	}
	if st.Avail != status.Up {
		return false, nil
	}
	return true, nil
}

func (t *T) Status(ctx context.Context) status.T {
	volume, err := t.Volume()
	if err != nil {
		t.StatusLog().Info("Volume %s does not exist (and no pool can provision it)", t.name())
		t.StatusLog().Info("%s", err)
		return status.Down
	}
	if !volume.Path().Exists() {
		t.StatusLog().Info("Volume %s does not exist", t.name())
		return status.Down
	}
	data, err := t.statusVolume(ctx, volume)
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	if data.Overall == status.Warn {
		t.StatusLog().Warn("Volume %s has warnings", volume.Path())
	}
	t.DataRecv.Status()
	if !t.flagInstalled() {
		if data.Avail == status.Warn {
			t.StatusLog().Error("%s avail %s", volume.Path(), data.Avail)
		} else {
			t.StatusLog().Info("%s avail %s", volume.Path(), data.Avail)
		}
		return status.Down
	}
	return data.Avail
}

func (t *T) flagFile() string {
	return filepath.Join(t.VarDir(), "flag")
}

func (t *T) flagInstalled() bool {
	return file.Exists(t.flagFile())
}

func (t *T) uninstallFlag() error {
	return os.Remove(t.flagFile())
}

func (t *T) installFlag() error {
	p := t.flagFile()
	if file.Exists(p) {
		return nil
	}
	d := filepath.Dir(p)
	if !file.Exists(d) {
		if err := os.MkdirAll(d, os.ModePerm); err != nil {
			return err
		}
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	f.Close()
	return nil
}

func (t *T) removeHolders(ctx context.Context) error {
	for _, dev := range t.ExposedDevices(ctx) {
		if err := dev.RemoveHolders(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) unclaim(volume object.Vol) error {
	volumeChildren, err := volume.Children()
	if err != nil {
		return err
	}
	if v, err := volumeChildren.HasPath(t.Path); err != nil {
		return err
	} else if !v {
		t.Log().Infof("volume %s is already unclaimed by %s", volume.Path(), t.Path)
		return nil
	}
	t.Log().Infof("unclaim volume %s", volume.Path())
	return volume.Config().Set(keyop.T{Key: key.Parse("children"), Op: keyop.Remove, Value: t.Path.String()})
}

func (t *T) incompatibleClaims(volumeChildren naming.Relations) []string {
	volumeChildrenNotInObjectParents := make(map[string]any)
	for _, volumeChild := range volumeChildren {
		volumeChildPath, err := volumeChild.Path()
		if err != nil {
			continue
		}
		rel1 := fmt.Sprintf("%s@%s", volumeChildPath, hostname.Hostname())
		rel2 := fmt.Sprintf("%s@%s", volumeChildPath.Name, hostname.Hostname())
		if !slices.Contains(t.ObjectParents, rel1) && !slices.Contains(t.ObjectParents, rel2) {
			volumeChildrenNotInObjectParents[volumeChildPath.String()] = nil
		}
	}
	l := make([]string, len(volumeChildrenNotInObjectParents))
	i := 0
	for k := range volumeChildrenNotInObjectParents {
		l[i] = k
		i++
	}
	return l
}

func (t *T) claim(volume object.Vol) error {
	volumeChildren, err := volume.Children()
	if err != nil {
		return err
	}
	if t.Shared {
		if v, err := volumeChildren.HasPath(t.Path); err != nil {
			return err
		} else if v {
			t.Log().Infof("shared volume %s is already claimed by %s", volume.Path(), t.Path)
			return nil
		}
		if l := t.incompatibleClaims(volumeChildren); len(l) > 0 {
			return fmt.Errorf("shared %s children %v must be local parents of %s to preserve placement affinity", volume.Path(), l, t.Path)
		}
		t.Log().Infof("shared volume %s current claims are compatible: %v", volume.Path(), volumeChildren)
	} else {
		if v, err := volumeChildren.HasPath(t.Path); err != nil {
			return err
		} else if v {
			t.Log().Infof("volume %s is already claimed by %s", volume.Path(), t.Path)
			return nil
		}
	}
	t.Log().Infof("claim volume %s", volume.Path())
	return volume.Config().Set(keyop.T{Key: key.Parse("children"), Op: keyop.Merge, Value: t.Path.String()})
}

func (t *T) access() volaccess.T {
	a, err := volaccess.Parse(t.Access)
	if err != nil {
		t.StatusLog().Warn("%s", err)
		a, _ = volaccess.Parse("rwo")
	}
	if t.Topology == topology.Flex {
		// translations: roo => rox, rwo => rwx
		a.SetOnce(false)
	}
	return a

}

// volumeLogger returns a logger that hints about this resource and object
// as the volume origin.
func (t *T) volumeLogger() *plog.Logger {
	return plog.NewDefaultLogger().Attr("from_obj_path", t.Path.String()).Attr("from_rid", t.ResourceID.String())
}

func (t *T) Volume() (object.Vol, error) {
	p, err := naming.NewPath(t.Path.Namespace, naming.KindVol, t.name())
	if err != nil {
		return nil, err
	}

	logger := t.volumeLogger()
	return object.NewVol(p, object.WithLogger(logger))
}

func (t *T) createVolume(ctx context.Context, volume object.Vol) (object.Vol, error) {
	if err := t.ValidateNodesAndName(); err != nil {
		return nil, err
	}
	p := filepath.Join(volume.Path().VarDir(), "create_volume.lock")
	lock := flock.New(p, xsession.ID.String(), fcntllock.New)
	if err := lock.Lock(20*time.Second, ""); err != nil {
		return nil, err
	}
	defer func() { _ = lock.UnLock() }()
	return t.lockedCreateVolume(ctx, volume)
}

func (t *T) lockedCreateVolume(ctx context.Context, volume object.Vol) (object.Vol, error) {
	volume.SetVolatile(false)
	err := t.configureVolume(ctx, volume, true)
	if err != nil {
		return nil, err
	}
	return volume, nil
}

// poolLookup exposes some methods like ConfigureVolume, which
// are relayed to the pool best matching the lookup criteria.
// The withUsage critierium can be toggled on/off because it
// may be slow to get fresh usage metrics, and only the
// provision codepath needs them (others are satisfied with the
// guarantee the pool is of the same type).
func (t *T) poolLookup(withUsage bool) (*pool.Lookup, error) {
	node, err := object.NewNode()
	if err != nil {
		return nil, err
	}
	l := pool.NewLookup(node)
	l.Name = t.Pool
	l.Type = t.PoolType
	if t.Size == nil {
		// unprovisionned volume should be able to access vol.Head()
		// avoid stacking in this situation.
		l.Size = 0.0
	} else {
		l.Size = *t.Size
	}
	l.Format = t.Format
	l.Shared = t.Shared
	l.Access, err = volaccess.Parse(t.Access)
	if err != nil {
		return nil, err
	}
	l.Nodes = t.VolNodes
	if withUsage {
		l.Usage = true
	}
	return l, err
}

func (t *T) volEnv() []string {
	return []string{}
}

func (t *T) configureVolume(ctx context.Context, v object.Vol, withUsage bool) error {
	l, err := t.poolLookup(withUsage)
	if err != nil {
		return err
	}
	logger := t.volumeLogger()
	obj, err := object.New(t.Path, object.WithLogger(logger))
	if err != nil {
		return err
	}
	return l.ConfigureVolume(ctx, v, obj)
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.Name
}

func (t *T) ValidateNodesAndName() error {
	m := make(map[string]string)
	for _, nodename := range t.VolNodes {
		m[nodename] = t.Name
	}
	obj := t.GetObject().(object.Configurer)
	k := key.T{
		Section: t.RID(),
		Option:  "name",
	}
	localhost := hostname.Hostname()
	for _, nodename := range t.Nodes {
		if nodename == localhost {
			continue
		}
		otherName, err := obj.Config().EvalAs(k, nodename)
		if err != nil {
			return err
		}
		if _, ok := m[nodename]; !ok && t.Name == otherName {
			return fmt.Errorf("%s conflicts with a volume of the same name on %s", t.Name, nodename)
		}
	}
	return nil
}

func (t *T) ProvisionAsFollower(ctx context.Context) error {
	volume, err := t.Volume()
	if err != nil {
		return err
	}
	if !volume.Path().Exists() {
		return fmt.Errorf("volume %s does not exist", t.Path)
	}
	if volumeStatus, err := volume.Status(ctx); err != nil {
		return err
	} else if volumeStatus.Provisioned == provisioned.True {
		t.Log().Infof("volume %s is already provisioned", volume.Path())
		return nil
	}
	return volume.Provision(ctx)
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	volume, err := t.Volume()
	if err != nil {
		return err
	}
	if !volume.Path().Exists() {
		if volume, err = t.createVolume(ctx, volume); err != nil {
			return err
		}
		// the volume resources cache is now wrong. Allocate a new one.
		volume, err = t.Volume()
		if err != nil {
			return err
		}
	} else {
		t.Log().Infof("volume %s is already created", volume.Path())
	}
	if err := t.claim(volume); err != nil {
		return err
	}
	if volumeStatus, err := volume.Status(ctx); err != nil {
		return err
	} else if volumeStatus.Provisioned == provisioned.True {
		t.Log().Infof("volume %s is already provisioned", volume.Path())
		return nil
	}
	return volume.Provision(ctx)
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	volume, err := t.Volume()
	if err != nil {
		return err
	}
	if !volume.Path().Exists() {
		t.Log().Infof("volume %s is already unprovisioned", volume.Path())
		return nil
	}
	if err := t.unclaim(volume); err != nil {
		return err
	}
	// don't unprovision vol objects (independent lifecycle)
	return nil
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	volume, err := t.Volume()
	if err != nil {
		return provisioned.False, err
	}
	exists := volume.Path().Exists()
	return provisioned.FromBool(exists), nil
}

func (t *T) Head() string {
	volume, err := t.Volume()
	if err != nil {
		return ""
	}
	return volume.Head()
}

func (t *T) exposedDevice(ctx context.Context) *device.T {
	volume, err := t.Volume()
	if err != nil {
		return nil
	}
	return volume.ExposedDevice(ctx)
}

func (t *T) ExposedDevices(ctx context.Context) device.L {
	volume, err := t.Volume()
	if err != nil {
		return nil
	}
	return volume.ExposedDevices(ctx)
}

func (t *T) SubDevices(ctx context.Context) device.L {
	volume, err := t.Volume()
	if err != nil {
		return nil
	}
	return volume.SubDevices(ctx)
}

// Configure installs a resource backpointer in the DataStoreInstall
func (t *T) Configure() error {
	t.DataRecv.SetReceiver(t)
	return nil
}

func (t *T) PreMove(ctx context.Context, to string) error {
	if t.IsDisabled() {
		return nil
	}
	volume, err := t.Volume()
	if err != nil {
		t.Log().Errorf("%s", err)
		return fmt.Errorf("volume %s does not exist (and no pool can create it)", t.name())
	}
	for _, r := range volume.Resources() {
		if r.IsDisabled() {
			continue
		}
		if i, ok := r.(resource.PreMover); ok {
			if err := i.PreMove(ctx, to); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *T) PreMoveRollback(ctx context.Context, to string) error {
	if t.IsDisabled() {
		return nil
	}
	volume, err := t.Volume()
	if err != nil {
		t.Log().Errorf("%s", err)
		return fmt.Errorf("volume %s does not exist (and no pool can create it)", t.name())
	}
	for _, r := range volume.Resources() {
		if r.IsDisabled() {
			continue
		}
		if i, ok := r.(resource.PreMoveRollbacker); ok {
			if err := i.PreMoveRollback(ctx, to); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *T) PostMove(ctx context.Context, to string) error {
	if t.IsDisabled() {
		return nil
	}
	volume, err := t.Volume()
	if err != nil {
		t.Log().Errorf("%s", err)
		return fmt.Errorf("volume %s does not exist (and no pool can create it)", t.name())
	}
	for _, r := range volume.Resources() {
		if r.IsDisabled() {
			continue
		}
		if i, ok := r.(resource.PostMover); ok {
			if err := i.PostMove(ctx, to); err != nil {
				return err
			}
		}
	}
	return nil
}
