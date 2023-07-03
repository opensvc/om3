package pooldirectory

import (
	"fmt"
	"path/filepath"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/util/df"
	"github.com/opensvc/om3/util/sizeconv"
)

type (
	T struct {
		pool.T
	}
)

var (
	drvID = driver.NewID(driver.GroupPool, "directory")
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

func (t T) Usage() (pool.StatusUsage, error) {
	entries, err := df.ContainingMountUsage(t.path())
	if err != nil {
		return pool.StatusUsage{}, err
	}
	if len(entries) == 0 {
		return pool.StatusUsage{}, fmt.Errorf("not mounted")
	}
	usage := pool.StatusUsage{
		Size: float64(entries[0].Total),
		Free: float64(entries[0].Free),
		Used: float64(entries[0].Used),
	}
	return usage, nil
}

func (t *T) loopFile(name string) string {
	return filepath.Join(t.path(), name+".img")
}

func (t *T) Translate(name string, size float64, shared bool) ([]string, error) {
	return []string{
		"fs#0.type=flag",
		"fs#1.type=directory",
		"fs#1.path=" + filepath.Join(t.path(), name),
	}, nil
}

func (t *T) BlkTranslate(name string, size float64, shared bool) ([]string, error) {
	return []string{
		"disk#0.type=loop",
		"disk#0.file=" + t.loopFile(name),
		"disk#0.size=" + sizeconv.ExactBSizeCompact(size),
	}, nil
}

func (t T) path() string {
	return t.GetString("path")
}
