//go:build linux

package poolvg

import (
	"context"
	"fmt"
	"strconv"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/util/lvm2"
)

type (
	T struct {
		pool.T
	}
)

var (
	drvID = driver.NewID(driver.GroupPool, "vg")
)

func init() {
	driver.Register(drvID, NewPooler)
}

func NewPooler() pool.Pooler {
	t := New()
	var i interface{} = t
	return i.(pool.Pooler)
}

func New() *T {
	t := T{}
	return &t
}

// Head is the volume group, or the thin pool in it the volumes are thin
// volumes of, named as the volumes say what they take from.
func (t T) Head() string {
	if thin := t.ThinPool(); thin != "" {
		return t.VGName() + "/" + thin
	}
	return t.VGName()
}

// Capabilities are the ones of a pool of logical volumes. A pool of thin
// volumes snapshots them, in their thin pool. A thick volume could be
// snapshotted too, but with a size of its own, which slows down its writes
// while the snapshot lives: that is not a capability to select a pool on.
func (t T) Capabilities() pool.Capabilities {
	l := pool.Capabilities{
		pool.CapBlk,
		pool.CapFile,
		pool.CapROO,
		pool.CapROX,
		pool.CapRWO,
		pool.CapRWX,
	}
	if t.ThinPool() != "" {
		l = append(l, pool.CapSnap)
	}
	return l
}

func (t T) VGName() string {
	return t.GetString("name")
}

// ThinPool is the thin pool of the volume group the volumes are made in,
// empty for thick volumes.
func (t T) ThinPool() string {
	return t.GetString("thin_pool")
}

// Usage is the one of the volume group, or of the thin pool the volumes are
// made in: its size, and the share of its data written to.
func (t T) Usage(ctx context.Context) (pool.Usage, error) {
	if thin := t.ThinPool(); thin != "" {
		return t.thinPoolUsage(ctx, thin)
	}
	vg := lvm2.NewVG(t.VGName())
	info, err := vg.Show(ctx, "vg_name,vg_free,vg_size")
	if err != nil {
		return pool.Usage{}, err
	}
	size, err := info.Size()
	if err != nil {
		return pool.Usage{}, err
	}
	free, err := info.Free()
	if err != nil {
		return pool.Usage{}, err
	}
	var used int64
	if size > 0 {
		used = size - free
	} else {
		size = 0
		free = 0
	}
	usage := pool.Usage{
		Size: size,
		Free: free,
		Used: used,
	}
	return usage, nil
}

func (t *T) Translate(name string, size int64, shared bool) ([]string, error) {
	data, err := t.BlkTranslate(name, size, shared)
	if err != nil {
		return nil, err
	}
	fs := pool.FS{
		Pool:      t,
		Name:      name,
		Shared:    shared,
		FsIndex:   1,
		DiskIndex: 0,
		OnDisk:    "disk#0",
	}
	data = append(data, fs.Keywords()...)
	return data, nil
}

func (t *T) BlkTranslate(name string, size int64, shared bool) ([]string, error) {
	data := []string{
		"disk#0.type=lv",
		"disk#0.name=" + name,
		"disk#0.vg=" + t.VGName(),
		"disk#0.size={DEFAULT.size}",
	}
	if thin := t.ThinPool(); thin != "" {
		data = append(data, "disk#0.thin_pool="+thin)
	}
	if opts := t.MkblkOptions(); opts != "" {
		data = append(data, "disk#0.create_options="+opts)
	}
	return data, nil
}

func (t T) path() string {
	return t.GetString("path")
}

func (t T) thinPoolUsage(ctx context.Context, name string) (pool.Usage, error) {
	info, err := lvm2.NewLV(t.VGName(), name).Show(ctx)
	if err != nil {
		return pool.Usage{}, err
	}
	size, err := strconv.ParseInt(info.LVSize, 10, 64)
	if err != nil {
		return pool.Usage{}, fmt.Errorf("thin pool %s/%s: parse lv_size %s: %w", t.VGName(), name, info.LVSize, err)
	}
	percent, err := strconv.ParseFloat(info.DataPercent, 64)
	if err != nil {
		return pool.Usage{}, fmt.Errorf("thin pool %s/%s: parse data_percent %s: %w", t.VGName(), name, info.DataPercent, err)
	}
	used := int64(float64(size) * percent / 100)
	return pool.Usage{
		Size: size,
		Free: size - used,
		Used: used,
	}, nil
}
