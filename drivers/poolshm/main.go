package poolshm

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

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
	drvID = driver.NewID(driver.GroupPool, "shm")
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

func (t T) Usage(ctx context.Context) (pool.Usage, error) {
	entries, err := df.MountUsage(ctx, t.path())
	if err != nil {
		return pool.Usage{}, err
	}
	if len(entries) == 0 {
		return pool.Usage{}, fmt.Errorf("not mounted")
	}
	usage := pool.Usage{
		Size: entries[0].Total,
		Free: entries[0].Free,
		Used: entries[0].Used,
	}
	return usage, nil
}

func (t *T) mntOpt(size int64) string {
	sizeOpt := "size=" + sizeconv.ExactBSizeCompact(float64(size))
	opts := t.GetString("mnt_opt")
	if opts == "" {
		opts = "mode=700"
	}
	opts = strings.Join([]string{opts, sizeOpt}, ",")
	return opts
}

func (t *T) loopFile(name string) string {
	return filepath.Join(t.path(), name+".img")
}

func (t *T) Translate(name string, size int64, shared bool) ([]string, error) {
	return []string{
		"fs#0.type=tmpfs",
		"fs#0.dev=none",
		"fs#0.mnt=" + pool.MountPointFromName(name),
		"fs#0.mnt_opt=" + t.mntOpt(size),
	}, nil
}

func (t *T) BlkTranslate(name string, size int64, shared bool) ([]string, error) {
	return []string{
		"disk#0.type=loop",
		"disk#0.file=" + t.loopFile(name),
		"disk#0.size=" + sizeconv.ExactBSizeCompact(float64(size)),
	}, nil
}
