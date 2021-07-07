package object

import (
	"context"

	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
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
func NewVol(p path.T, opts ...funcopt.O) *Vol {
	s := &Vol{}
	s.Base.init(p, opts...)
	return s
}

func (t *Vol) MountPoint() string {
	return ""
}

func (t *Vol) Device() *device.T {
	return nil
}

func (t *Vol) HoldersExcept(ctx context.Context, p path.T) path.L {
	l := make([]path.T, 0)
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
		i := NewFromPath(p, WithVolatile(true))
		o, ok := i.(ResourceLister)
		if !ok {
			continue
		}
		for _, r := range o.Resources() {
			if r.ID().DriverGroup() != drivergroup.Volume {
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
	return path.L(l)
}
