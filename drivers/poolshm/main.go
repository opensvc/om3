package poolshm

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/util/df"
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

func (t T) Capabilities() pool.Capabilities {
	return pool.Capabilities{
		pool.CapBlk,
		pool.CapFile,
		pool.CapROO,
		pool.CapROX,
		pool.CapRWO,
		pool.CapRWX,
		pool.CapVolatile,
	}
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

// defaultMode is the mode of the root of a volume, unless the pool says
// otherwise: the volume is for the objects using it, where the root of a
// tmpfs is everyone's to write in by default.
const defaultMode = "700"

// mode is the mode of the root of the volumes the pool makes: the mode
// keyword, or else a mode= of mnt_opt, which is where pools configured before
// the keyword say it.
func (t *T) mode() string {
	if s := t.GetString("mode"); s != "" {
		return s
	}
	for _, option := range strings.Split(t.GetString("mnt_opt"), ",") {
		if s, ok := strings.CutPrefix(option, "mode="); ok && s != "" {
			return s
		}
	}
	return defaultMode
}

// mntOpt is the mount options of the volumes the pool makes, less the size
// and the mode, which the volume says in keywords of its own.
func (t *T) mntOpt() string {
	l := make([]string, 0)
	for _, option := range strings.Split(t.GetString("mnt_opt"), ",") {
		switch {
		case option == "":
		case strings.HasPrefix(option, "size="):
		case strings.HasPrefix(option, "mode="):
		default:
			l = append(l, option)
		}
	}
	return strings.Join(l, ",")
}

func (t *T) loopFile(name string) string {
	return filepath.Join(t.path(), name+".img")
}

// Translate returns the configuration of a volume the pool makes, a tmpfs.
//
// Its size is DEFAULT.size, the size the volume is claimed with, by
// reference: a resize of the volume records the size it reached there, and
// every later mount reads it, where a size written in mnt_opt kept the one
// the volume was made with, so the next mount undid every resize.
func (t *T) Translate(name string, size int64, shared bool) ([]string, error) {
	l := []string{
		"fs#0.type=tmpfs",
		"fs#0.dev=none",
		"fs#0.mnt=" + pool.MountPointFromName(name),
		"fs#0.size={DEFAULT.size}",
		"fs#0.mode=" + t.mode(),
	}
	if s := t.mntOpt(); s != "" {
		l = append(l, "fs#0.mnt_opt="+s)
	}
	return l, nil
}

func (t *T) BlkTranslate(name string, size int64, shared bool) ([]string, error) {
	return []string{
		"disk#0.type=loop",
		"disk#0.file=" + t.loopFile(name),
		"disk#0.size={DEFAULT.size}",
	}, nil
}
