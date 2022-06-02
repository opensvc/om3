//go:build linux
// +build linux

package poolvg

import (
	"strings"

	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/pool"
	"opensvc.com/opensvc/util/lvm2"
	"opensvc.com/opensvc/util/sizeconv"
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

func (t T) Usage() (pool.StatusUsage, error) {
	vg := lvm2.NewVG(t.VGName())
	info, err := vg.Show("vg_name,vg_free,vg_size")
	if err != nil {
		return pool.StatusUsage{}, err
	}
	size, err := sizeconv.FromSize(strings.TrimLeft(info.VGSize, "<>+"))
	if err != nil {
		return pool.StatusUsage{}, err
	}
	free, err := sizeconv.FromSize(strings.TrimLeft(info.VGFree, "<>+"))
	if err != nil {
		return pool.StatusUsage{}, err
	}
	var used int64
	if size > 0 {
		size = size / 1024
		free = free / 1024
		used = size - free
	} else {
		size = 0
		free = 0
	}
	usage := pool.StatusUsage{
		Size: float64(size),
		Free: float64(free),
		Used: float64(used),
	}
	return usage, nil
}

func (t *T) Translate(name string, size float64, shared bool) []string {
	data := t.BlkTranslate(name, size, shared)
	data = append(data, t.AddFS(name, shared, 1, 0, "disk#0")...)
	return data
}

func (t *T) BlkTranslate(name string, size float64, shared bool) []string {
	data := []string{
		"disk#0.type=lv",
		"disk#0.name=" + name,
		"disk#0.vg=" + t.VGName(),
		"disk#0.size=" + sizeconv.ExactBSizeCompact(size),
	}
	if opts := t.MkblkOptions(); opts != "" {
		data = append(data, "disk#0.create_options="+opts)
	}
	return data
}

func (t T) path() string {
	return t.GetString("path")
}
