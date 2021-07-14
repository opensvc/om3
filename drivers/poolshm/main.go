package poolshm

import (
	"fmt"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/core/pool"
	"opensvc.com/opensvc/util/df"
	"opensvc.com/opensvc/util/sizeconv"
)

type (
	T struct {
		pool.T
	}
)

func init() {
	pool.Register("shm", NewPooler)
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
	entries, err := df.MountUsage(t.path())
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

func (t *T) mntOpt(size float64) string {
	sizeOpt := "size=" + sizeconv.ExactBSizeCompact(size)
	opts := t.GetString("mnt_opt")
	if opts != "" {
		opts = strings.Join([]string{opts, sizeOpt}, ",")
	} else {
		opts = sizeOpt
	}
	return opts
}

func (t *T) loopFile(name string) string {
	return filepath.Join(t.path(), name+".img")
}

func (t *T) Translate(name string, size float64, shared bool) []string {
	return []string{
		"fs#0.type=tmpfs",
		"fs#0.dev=none",
		"fs#0.mnt=" + pool.MountPointFromName(name),
		"fs#0.mnt_opt=" + t.mntOpt(size),
	}
}

func (t *T) BlkTranslate(name string, size float64, shared bool) []string {
	return []string{
		"disk#0.type=loop",
		"disk#0.file=" + t.loopFile(name),
		"disk#0.size=" + sizeconv.ExactBSizeCompact(size),
	}
}
