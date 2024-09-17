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
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/core/volaccess"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/xsession"
)

type (
	T struct {
		resource.T
		Name        string       `json:"name"`
		Access      string       `json:"access"`
		Pool        string       `json:"pool"`
		PoolType    string       `json:"type"`
		Size        *int64       `json:"size"`
		Format      bool         `json:"format"`
		Configs     []string     `json:"configs"`
		Secrets     []string     `json:"secrets"`
		Directories []string     `json:"directories"`
		User        string       `json:"user"`
		Group       string       `json:"group"`
		Perm        *os.FileMode `json:"perm"`
		DirPerm     *os.FileMode `json:"dirperm"`
		Signal      string       `json:"signal"`
		VolNodes    []string

		Path     naming.Path
		Topology topology.T
		Nodes    []string
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

func (t T) startVolume(ctx context.Context, volume object.Vol) error {
	return volume.Start(ctx)
}

func (t T) stopVolume(ctx context.Context, volume object.Vol, force bool) error {
	ctx = actioncontext.WithForce(ctx, true)
	holders := volume.HoldersExcept(ctx, t.Path)
	if len(holders) > 0 {
		t.Log().Infof("skip volume %s stop: active users: %s", volume.Path(), holders)
		return nil
	}
	return volume.Stop(ctx)
}

func (t T) statusVolume(ctx context.Context, volume object.Vol) (instance.Status, error) {
	return volume.FreshStatus(ctx)
}

func (t T) Start(ctx context.Context) error {
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
	actionrollback.Register(ctx, func() error {
		return t.stopVolume(ctx, volume, false)
	})
	if err = t.startFlag(ctx); err != nil {
		return err
	}
	if err = t.installData(); err != nil {
		return err
	}
	return nil
}

func (t T) stopFlag(ctx context.Context) error {
	if !t.flagInstalled() {
		return nil
	}
	if err := t.uninstallFlag(); err != nil {
		return err
	}
	return nil
}

func (t T) startFlag(ctx context.Context) error {
	if t.flagInstalled() {
		return nil
	}
	if err := t.installFlag(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.uninstallFlag()
	})
	return nil
}

func (t T) Stop(ctx context.Context) error {
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

func (t T) name() string {
	if t.Name != "" {
		return t.Name
	}
	return t.Path.Name + "-vol-" + t.ResourceID.Index()
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
		t.StatusLog().Error("Volume %s has warnings", volume.Path())
	}
	t.statusData()
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

func (t T) flagFile() string {
	return filepath.Join(t.VarDir(), "flag")
}

func (t T) flagInstalled() bool {
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

func (t T) removeHolders() error {
	return t.exposedDevice().RemoveHolders()
}

func (t T) access() volaccess.T {
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
	v, err := object.NewVol(p, object.WithLogger(logger))
	if err != nil {
		return nil, err
	}
	if !p.Exists() {
		v.SetVolatile(true)
		if err := t.configureVolume(v, false); err != nil {
			return nil, err
		}
	}
	return v, nil
}

func (t *T) createVolume(volume object.Vol) (object.Vol, error) {
	if err := t.ValidateNodesAndName(); err != nil {
		return nil, err
	}
	p := filepath.Join(volume.Path().VarDir(), "create_volume.lock")
	lock := flock.New(p, xsession.ID.String(), fcntllock.New)
	timeout, err := time.ParseDuration("20s")
	if err != nil {
		return nil, err
	}
	err = lock.Lock(timeout, "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = lock.UnLock() }()
	return t.lockedCreateVolume(volume)
}

func (t *T) lockedCreateVolume(volume object.Vol) (object.Vol, error) {
	volume.SetVolatile(false)
	err := t.configureVolume(volume, true)
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
// garanty the pool is of the same type).
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

func (t *T) configureVolume(v object.Vol, withUsage bool) error {
	l, err := t.poolLookup(withUsage)
	if err != nil {
		return err
	}
	logger := t.volumeLogger()
	obj, err := object.New(t.Path, object.WithLogger(logger))
	if err != nil {
		return err
	}
	return l.ConfigureVolume(v, obj)
}

func (t T) Label() string {
	return t.Name
}

func (t T) ValidateNodesAndName() error {
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

func (t T) ProvisionLeaded(ctx context.Context) error {
	volume, err := t.Volume()
	if err != nil {
		return err
	}
	if !volume.Path().Exists() {
		if t.IsShared() {
			return fmt.Errorf("shared volume %s does not exists", volume.Path())
		}
		if volume, err = t.createVolume(volume); err != nil {
			return err
		}
		// the volume resources cache is now wrong. Allocate a new one.
		volume, err = t.Volume()
		if err != nil {
			return err
		}
	}
	return volume.Provision(ctx)
}

func (t T) UnprovisionLeaded(ctx context.Context) error {
	volume, err := t.Volume()
	if err != nil {
		return err
	}
	if !volume.Path().Exists() {
		t.Log().Infof("volume %s is already unprovisioned", volume.Path())
		return nil
	}
	return nil
}

func (t T) ProvisionLeader(ctx context.Context) error {
	volume, err := t.Volume()
	if err != nil {
		return err
	}
	if !volume.Path().Exists() {
		if volume, err = t.createVolume(volume); err != nil {
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
	return volume.Provision(ctx)
}

func (t T) UnprovisionLeader(ctx context.Context) error {
	volume, err := t.Volume()
	if err != nil {
		return err
	}
	if !volume.Path().Exists() {
		t.Log().Infof("volume %s is already unprovisioned", volume.Path())
		return nil
	}
	// don't unprovision vol objects (independent lifecycle)
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	volume, err := t.Volume()
	if err != nil {
		return provisioned.False, err
	}
	exists := volume.Path().Exists()
	return provisioned.FromBool(exists), nil
}

func (t T) Head() string {
	volume, err := t.Volume()
	if err != nil {
		return ""
	}
	return volume.Head()
}

func (t T) exposedDevice() *device.T {
	volume, err := t.Volume()
	if err != nil {
		return nil
	}
	return volume.Device()
}

func (t T) ExposedDevices() device.L {
	dev := t.exposedDevice()
	if dev == nil {
		return device.L{}
	}
	return device.L{*dev}
}
