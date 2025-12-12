//go:build linux

package poolloop

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/util/df"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	T struct {
		pool.T
	}
)

var (
	drvID = driver.NewID(driver.GroupPool, "loop")
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
	return t.path()
}

func (t T) Capabilities() []string {
	return []string{"rox", "rwx", "roo", "rwo", "blk"}
}

func (t T) Usage() (pool.Usage, error) {
	head := t.Head()
	entries, err := df.ContainingMountUsage(head)
	if err != nil {
		return pool.Usage{}, err
	}
	if len(entries) == 0 {
		return pool.Usage{}, err
	}
	e := entries[0]
	var size, free, used int64
	if e.Total > 0 {
		size = e.Total
		free = e.Free
		used = e.Used
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
	p := fmt.Sprintf("%s/%s.img", t.Head(), name)
	data := []string{
		"disk#0.type=loop",
		"disk#0.name=" + name,
		"disk#0.size=" + sizeconv.ExactBSizeCompact(float64(size)),
		"disk#0.file=" + p,
	}
	return data, nil
}

func (t T) path() string {
	return t.GetString("path")
}
