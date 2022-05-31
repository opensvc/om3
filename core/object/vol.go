package object

import (
	"context"
	"sort"

	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
)

type (
	//
	// Vol is the vol-kind object.
	//
	// These objects contain cluster-dependent fs, disk and sync resources.
	//
	// They are created by feeding a volume resource configuration (cluster
	// independant) to a pool.
	//
	Vol struct {
		Base
	}
)

// NewVol allocates a vol kind object.
func NewVol(p path.T, opts ...funcopt.O) (*Vol, error) {
	s := &Vol{}
	err := s.Base.init(s, p, opts...)
	return s, err
}

func (t Vol) KeywordLookup(k key.T, sectionType string) keywords.Keyword {
	return keywordLookup(keywordStore, k, t.Path.Kind, sectionType)
}

//
// Head returns the shortest service fs resource mount point.
// Volume resources in the consumer services use this function return
// value as the prefix of their own mount_point property.
//
// The candidates are sort from shallowest to deepest mountpoint, so
// the shallowest candidate is returned.
//
// Callers must check the returned value is not empty.
//
func (t *Vol) Head() string {
	head := ""
	heads := make([]string, 0)
	type header interface {
		Head() string
	}
	drvgrps := []driver.Group{
		driver.GroupFS,
		driver.GroupVolume,
	}
	l := ResourcesByDrivergroups(t, drvgrps)
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

func (t *Vol) Device() *device.T {
	type devicer interface {
		ExposedDevices() []*device.T
	}
	rids := make([]string, 0)
	candidates := make(map[string]devicer)
	l := ResourcesByDrivergroups(t, []driver.Group{
		driver.GroupDisk,
		driver.GroupVolume,
	})
	for _, r := range l {
		if r.Manifest().Name == "scsireserv" {
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
		return devs[0]
	}
	return nil
}

func (t *Vol) HoldersExcept(ctx context.Context, p path.T) path.L {
	l := make(path.L, 0)
	type VolNamer interface {
		VolName() string
	}
	for _, rel := range t.Children() {
		p, node, err := rel.Split()
		if err != nil {
			continue
		}
		if node != "" && node != hostname.Hostname() {
			continue
		}
		i, err := NewFromPath(p, WithVolatile(true))
		if err != nil {
			t.log.Error().Err(err).Msg("")
			continue
		}
		o, ok := i.(ResourceLister)
		if !ok {
			continue
		}
		for _, r := range o.Resources() {
			if r.ID().DriverGroup() != driver.GroupVolume {
				continue
			}
			if o, ok := r.(VolNamer); ok {
				if o.VolName() != t.Path.Name {
					continue
				}
			}
			switch r.Status(ctx) {
			case status.Up, status.Warn:
				l = append(l, p)
			}
		}

	}
	return l
}
