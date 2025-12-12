//go:build linux

package poolvg

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/util/lvm2"
	"github.com/opensvc/om3/v3/util/sizeconv"
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

func (t T) Head() string {
	return t.VGName()
}

func (t T) Capabilities() []string {
	return []string{"rox", "rwx", "roo", "rwo", "blk"}
}

func (t T) VGName() string {
	return t.GetString("name")
}

func (t T) Usage() (pool.Usage, error) {
	vg := lvm2.NewVG(t.VGName())
	info, err := vg.Show("vg_name,vg_free,vg_size")
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
	data = append(data, t.AddFS(name, shared, 1, 0, "disk#0")...)
	return data, nil
}

func (t *T) BlkTranslate(name string, size int64, shared bool) ([]string, error) {
	data := []string{
		"disk#0.type=lv",
		"disk#0.name=" + name,
		"disk#0.vg=" + t.VGName(),
		"disk#0.size=" + sizeconv.ExactBSizeCompact(float64(size)),
	}
	if opts := t.MkblkOptions(); opts != "" {
		data = append(data, "disk#0.create_options="+opts)
	}
	return data, nil
}

func (t T) path() string {
	return t.GetString("path")
}
