/*
Volume resource driver

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
	"os/user"
	"path/filepath"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/pool"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/core/volaccess"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/xsession"
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
		User        *user.User   `json:"user"`
		Group       *user.Group  `json:"group"`
		Perm        *os.FileMode `json:"perm"`
		DirPerm     *os.FileMode `json:"dirperm"`
		Signal      string       `json:"signal"`

		Path     path.T
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

func (t T) startVolume(ctx context.Context, volume *object.Vol) error {
	options := object.OptsStart{}
	//options.Leader = actioncontext.IsLeader(ctx)
	return volume.Start(options)
}

func (t T) stopVolume(ctx context.Context, volume *object.Vol, force bool) error {
	options := object.OptsStop{}
	options.Force = force
	//options.Leader = actioncontext.IsLeader(ctx)
	holders := volume.HoldersExcept(ctx, t.Path)
	if len(holders) > 0 {
		t.Log().Info().Msgf("skip %s stop: active users: %s", volume.Path(), holders)
		return nil
	}
	return volume.Stop(options)
}

func (t T) statusVolume(ctx context.Context, volume *object.Vol) (instance.Status, error) {
	options := object.OptsStatus{}
	ctxOptions := actioncontext.Options(ctx)
	if i, ok := ctxOptions.(object.OptsStatus); ok {
		options.Refresh = i.Refresh
	} else {
		options.Refresh = true
	}
	return volume.Status(options)
}

func (t T) Start(ctx context.Context) error {
	volume, err := t.Volume()
	if err != nil {
		t.Log().Error().Err(err).Msg("")
		return fmt.Errorf("volume %s does not exist (and no pool can create it)", t.name())
	}
	if !object.Exists(volume.Path()) {
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
		t.StatusLog().Info("vol %s does not exist (and no pool can provision it)", t.name())
		t.StatusLog().Info("%s", err)
		return status.Down
	}
	if !object.Exists(volume.Path()) {
		t.StatusLog().Info("vol %s does not exist", t.name())
		return status.Down
	}
	data, err := t.statusVolume(ctx, volume)
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	if data.Overall == status.Warn {
		t.StatusLog().Error("vol %s has warnings", volume.Path())
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

func (t *T) Volume() (*object.Vol, error) {
	p, err := path.New(t.name(), t.Path.Namespace, kind.Vol.String())
	if err != nil {
		return nil, err
	}
	v, err := object.NewVol(p)
	if err != nil {
		return nil, err
	}
	if !object.Exists(p) {
		v.SetVolatile(true)
		if err := t.configureVolume(v, false); err != nil {
			return nil, err
		}
	}
	return v, nil
}

func (t *T) createVolume(volume *object.Vol) (*object.Vol, error) {
	p := filepath.Join(object.VarDir(volume.Path()), "create_volume.lock")
	lock := flock.New(p, xsession.ID, fcntllock.New)
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

func (t *T) lockedCreateVolume(volume *object.Vol) (*object.Vol, error) {
	volume.SetVolatile(false)
	err := t.configureVolume(volume, true)
	if err != nil {
		return nil, err
	}
	return volume, nil
}

//
// poolLookup exposes some methods like ConfigureVolume, which
// are relayed to the pool best matching the lookup criteria.
// The withUsage critierium can be toggled on/off because it
// may be slow to get fresh usage metrics, and only the
// provision codepath needs them (others are satisfied with the
// garanty the pool is of the same type).
//
func (t *T) poolLookup(withUsage bool) (*pool.Lookup, error) {
	var err error
	node := object.NewNode() // TODO: find a more efficient method
	l := pool.NewLookup(node)
	l.Name = t.Pool
	l.Type = t.PoolType
	if t.Size == nil {
		// unprovisionned volume should be able to access vol.Head()
		// avoid stacking in this situation.
		l.Size = 0.0
	} else {
		l.Size = float64(*t.Size)
	}
	l.Format = t.Format
	l.Shared = t.Shared
	l.Access, err = volaccess.Parse(t.Access)
	if err != nil {
		return nil, err
	}
	if withUsage {
		l.Usage = true
	}
	return l, err
}

func (t *T) volEnv() []string {
	return []string{}
}

func (t *T) configureVolume(v *object.Vol, withUsage bool) error {
	l, err := t.poolLookup(withUsage)
	if err != nil {
		return err
	}
	obj, err := object.NewFromPath(t.Path) // TODO: find a more efficient method
	if err != nil {
		return err
	}
	return l.ConfigureVolume(v, obj)
}

func (t T) Label() string {
	return t.Name
}

func (t T) ProvisionLeader(ctx context.Context) error {
	volume, err := t.Volume()
	if err != nil {
		return err
	}
	if object.Exists(volume.Path()) {
		t.Log().Info().Msgf("%s is already provisioned", volume.Path())
		return nil
	}
	if volume, err = t.createVolume(volume); err != nil {
		return err
	}
	return volume.Provision(object.OptsProvision{})
}

func (t T) UnprovisionLeader(ctx context.Context) error {
	volume, err := t.Volume()
	if err != nil {
		return err
	}
	if !object.Exists(volume.Path()) {
		t.Log().Info().Msgf("%s is already unprovisioned", volume.Path())
		return nil
	}
	return volume.Unprovision(object.OptsUnprovision{})
}

func (t T) Provisioned() (provisioned.T, error) {
	volume, err := t.Volume()
	if err != nil {
		return provisioned.False, err
	}
	exists := object.Exists(volume.Path())
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

func (t T) ExposedDevices() []*device.T {
	dev := t.exposedDevice()
	if dev == nil {
		return []*device.T{}
	}
	return []*device.T{dev}
}
