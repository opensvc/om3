package object

import (
	"context"
	"sort"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/volaccess"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
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
		Device() *device.T
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

func (t *vol) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
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

func (t *vol) Device() *device.T {
	type devicer interface {
		ExposedDevices() device.L
	}
	rids := make([]string, 0)
	candidates := make(map[string]devicer)
	l := t.ResourcesByDrivergroups([]driver.Group{
		driver.GroupDisk,
		driver.GroupVolume,
	})
	for _, r := range l {
		if r.Manifest().DriverID.Name == "scsireserv" {
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
	sort.Strings(rids)
	for _, rid := range rids {
		devs := candidates[rid].ExposedDevices()
		if len(devs) == 0 {
			continue
		}
		return &devs[0]
	}
	return nil
}

func (t *vol) HoldersExcept(ctx context.Context, p naming.Path) (naming.Paths, error) {
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
	return naming.ParseRelations(l), nil
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
