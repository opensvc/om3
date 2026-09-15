package object

import (
	"context"
	"fmt"
	"sort"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/volaccess"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	vol struct {
		actor
	}

	//
	// Vol is the vol-kind object.
	//
	// These objects contain cluster-dependent fs, disk and sync resources.
	//
	// They are created by feeding a volume resource configuration (cluster
	// independent) to a pool.
	//
	Vol interface {
		Actor
		Head() string
		HeadRID(context.Context) (string, error)
		ConfiguredSize() (int64, error)
		PoolName() (string, error)
		ExposedDevice(context.Context) *device.T
		ExposedDevices(context.Context) device.L
		SubDevice(context.Context) *device.T
		SubDevices(context.Context) device.L
		HoldersExcept(ctx context.Context, p naming.Path) (naming.Paths, error)
		Access() (volaccess.T, error)
		Children() (naming.Relations, error)
	}
)

// NewVol allocates a vol kind object.
func NewVol(path naming.Path, opts ...funcopt.O) (*vol, error) {
	s := &vol{}
	s.path = path
	s.path.Kind = naming.KindVol
	err := s.init(s, path, opts...)
	return s, err
}

func (t *vol) KeywordLookup(k key.T, sectionType string) *keywords.Keyword {
	return keywordLookup(keywordStore, k, t.path.Kind, sectionType)
}

// Head returns the shortest service fs resource mount point.
// Volume resources in the consumer services use this function return
// value as the prefix of their own mount_point property.
//
// The candidates are sorted from shallowest to deepest mountpoint, so
// the shallowest candidate is returned.
//
// Callers must check the returned value is not empty.
func (t *vol) Head() string {
	head := ""
	heads := make([]string, 0)
	type header interface {
		Head() string
	}
	l := t.ResourcesByDrivergroups([]driver.Group{
		driver.GroupFS,
		driver.GroupVolume,
	})
	for _, r := range l {
		var i interface{} = r
		o, ok := i.(header)
		if !ok {
			continue
		}
		heads = append(heads, o.Head())
	}
	switch len(heads) {
	case 0:
		head = ""
	case 1:
		head = heads[0]
	default:
		sort.Strings(heads)
		head = heads[0]
	}
	return head
}

func (t *vol) SubDevices(ctx context.Context) device.L {
	type devicer interface {
		SubDevices(context.Context) device.L
	}
	rids := t.config.GetStrings(key.Parse("devices_from"))
	devs := make(device.L, 0)
	if len(rids) == 0 {
		dev := t.SubDevice(ctx)
		if dev != nil {
			devs = append(devs, *dev)
		}
		return devs
	}
	t.ConfigureResources()
	for _, rid := range rids {
		r := t.ResourceByID(rid)
		if r == nil {
			continue
		}
		if d, ok := r.(devicer); ok {
			devs = append(devs, d.SubDevices(ctx)...)
		}
	}
	return devs
}

func (t *vol) ExposedDevices(ctx context.Context) device.L {
	type devicer interface {
		ExposedDevices(context.Context) device.L
	}
	rids := t.config.GetStrings(key.Parse("devices_from"))
	devs := make(device.L, 0)
	if len(rids) == 0 {
		dev := t.ExposedDevice(ctx)
		if dev != nil {
			devs = append(devs, *dev)
		}
		return devs
	}
	t.ConfigureResources()
	for _, rid := range rids {
		r := t.ResourceByID(rid)
		if r == nil {
			continue
		}
		if d, ok := r.(devicer); ok {
			devs = append(devs, d.ExposedDevices(ctx)...)
		}
	}
	return devs
}

func (t *vol) SubDevice(ctx context.Context) *device.T {
	type devicer interface {
		SubDevices(context.Context) device.L
	}
	rids := make([]string, 0)
	candidates := make(map[string]devicer)
	l := t.ResourcesByDrivergroups([]driver.Group{
		driver.GroupDisk,
		driver.GroupVolume,
	})
	for _, r := range l {
		if r.DriverID().Name == "scsireserv" {
			continue
		}
		var i interface{} = r
		o, ok := i.(devicer)
		if !ok {
			continue
		}
		rid := r.RID()
		candidates[rid] = o
		rids = append(rids, rid)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(rids)))
	for _, rid := range rids {
		devs := candidates[rid].SubDevices(ctx)
		if len(devs) == 0 {
			continue
		}
		return &devs[0]
	}
	return nil
}

func (t *vol) ExposedDevice(ctx context.Context) *device.T {
	_, devs := t.exposedDeviceResource(ctx)
	if len(devs) == 0 {
		return nil
	}
	return &devs[0]
}

// exposedDeviceResource returns the resource the volume exposes a device
// through, and the devices that resource exposes.
//
// The deepest rid wins, so the resource nearest the consumer is the one
// named.
func (t *vol) exposedDeviceResource(ctx context.Context) (resource.Driver, device.L) {
	type devicer interface {
		ExposedDevices(context.Context) device.L
	}
	rids := make([]string, 0)
	candidates := make(map[string]resource.Driver)
	l := t.ResourcesByDrivergroups([]driver.Group{
		driver.GroupDisk,
		driver.GroupVolume,
	})
	for _, r := range l {
		if r.DriverID().Name == "scsireserv" {
			continue
		}
		var i interface{} = r
		if _, ok := i.(devicer); !ok {
			continue
		}
		rid := r.RID()
		candidates[rid] = r
		rids = append(rids, rid)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(rids)))
	for _, rid := range rids {
		r := candidates[rid]
		var i interface{} = r
		devs := i.(devicer).ExposedDevices(ctx)
		if len(devs) == 0 {
			continue
		}
		return r, devs
	}
	return nil, nil
}

// PoolName is the pool the volume was claimed from, and "" for a volume
// claimed from no pool.
func (t *vol) PoolName() (string, error) {
	return t.config.GetString(key.T{Section: "DEFAULT", Option: "pool"}), nil
}

// ConfiguredSize is the size the volume is asked to be.
//
// It is what was claimed of the pool when the volume was created, and what a
// resize writes back, so it is the size the volume is meant to hold rather
// than a record of what it held once.
func (t *vol) ConfiguredSize() (int64, error) {
	size := t.config.GetSize(key.T{Section: "DEFAULT", Option: "size"})
	if size == nil {
		return 0, fmt.Errorf("%s has no size", t.path)
	}
	return *size, nil
}

// HeadRID returns the rid of the resource a volume exposes to its consumers.
//
// A volume exists to expose one thing: the filesystem mounted on Head(), or,
// when the volume has no filesystem, the device it exposes. An action asked of
// the volume itself, like a resize, is an action on that resource.
func (t *vol) HeadRID(ctx context.Context) (string, error) {
	type header interface {
		Head() string
	}
	if head := t.Head(); head != "" {
		l := t.ResourcesByDrivergroups([]driver.Group{
			driver.GroupFS,
			driver.GroupVolume,
		})
		for _, r := range l {
			var i interface{} = r
			o, ok := i.(header)
			if !ok {
				continue
			}
			if o.Head() == head {
				return r.RID(), nil
			}
		}
	}
	if r, _ := t.exposedDeviceResource(ctx); r != nil {
		return r.RID(), nil
	}
	return "", fmt.Errorf("%s exposes neither a head mount point nor a device", t.path)
}

func (t *vol) HoldersExcept(ctx context.Context, exceptPath naming.Path) (naming.Paths, error) {
	l := make(naming.Paths, 0)
	type volNamer interface {
		VolName() string
	}
	children, err := t.Children()
	if err != nil {
		return l, err
	}
	for _, rel := range children {
		p, node, err := rel.Split()
		if err != nil {
			t.log.Errorf("%s", err)
			continue
		}
		if p == exceptPath {
			continue
		}
		if node != "" && node != hostname.Hostname() {
			continue
		}
		i, err := New(p, WithVolatile(true))
		if err != nil {
			t.log.Errorf("%s", err)
			continue
		}
		o, ok := i.(resourceLister)
		if !ok {
			continue
		}
		for _, r := range o.Resources() {
			if r.ID().DriverGroup() != driver.GroupVolume {
				continue
			}
			if o, ok := r.(volNamer); ok {
				if o.VolName() != t.path.Name {
					continue
				}
			}
			if resourceStatus := r.Status(ctx); resourceStatus.Is(status.Down, status.StandbyDown, status.NotApplicable, status.Undef) {
				continue
			}
			l = append(l, p)
		}

	}
	return l, nil
}

func (t *vol) Children() (naming.Relations, error) {
	k := key.Parse("children")
	l, err := t.config.GetStringsStrict(k)
	if err != nil {
		t.log.Errorf("%s", err)
		return naming.Relations{}, err
	}
	return naming.ParseRelations(l, t.Path().Namespace), nil
}

// Access returns the volaccess.Parse result of volume kw 'access'.
func (t *vol) Access() (volaccess.T, error) {
	k := key.Parse("access")
	if s, err := t.config.GetStringStrict(k); err != nil {
		return volaccess.T{}, err
	} else {
		return volaccess.Parse(s)
	}
}
