// Package poolshare is the pool of the volumes in a directory every node
// mounts from the same server, as a nfs export: the share pool of v2.
//
// A volume is a directory of the share, or a loop file there for a block
// volume. A directory has no size of its own, and the project quota of the
// directory pool does not reach through nfs, so the pool bounds no volume and
// has no quota capability: a namespace claiming it is refused the volumes it
// asks of it, as of any pool not bounding them.
package poolshare

import (
	"context"
	"fmt"
	"path/filepath"

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
	drvID = driver.NewID(driver.GroupPool, "share")
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

// Capabilities are the ones of v2: every access mode, block volumes, and
// shared, the share being the same on every node.
func (t T) Capabilities() pool.Capabilities {
	return pool.Capabilities{
		pool.CapBlk,
		pool.CapFile,
		pool.CapROO,
		pool.CapROX,
		pool.CapRWO,
		pool.CapRWX,
		pool.CapShared,
	}
}

// Usage is the one of the filesystem of the share. A path on a local
// filesystem, the share not mounted on the node, has none: the usage of the
// filesystem holding the mount point would have the pool serve volumes the
// other nodes do not see.
func (t T) Usage(ctx context.Context) (pool.Usage, error) {
	if err := checkShared(t.path()); err != nil {
		return pool.Usage{}, err
	}
	entries, err := df.ContainingMountUsage(ctx, t.path())
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

// volumePath is where the volume name lives on the share: its own path when
// shared by the nodes, else a path per node, as v2 laid them out.
func (t *T) volumePath(name string, shared bool) string {
	if shared {
		return filepath.Join(t.path(), name)
	}
	return filepath.Join(t.path(), name+".{nodename}")
}

// Translate makes the volume a directory of the share. A volume shared by the
// nodes has its directory shared: one node provisions and unprovisions it,
// the others leaving alone what another node uses.
func (t *T) Translate(name string, size int64, shared bool) ([]string, error) {
	return []string{
		"fs#0.type=flag",
		"fs#1.type=directory",
		"fs#1.path=" + t.volumePath(name, shared),
		"fs#1.shared=" + fmt.Sprint(shared),
	}, nil
}

func (t *T) BlkTranslate(name string, size int64, shared bool) ([]string, error) {
	return []string{
		"disk#0.type=loop",
		"disk#0.file=" + t.volumePath(name, shared) + ".img",
		"disk#0.size={DEFAULT.size}",
		"disk#0.shared=" + fmt.Sprint(shared),
	}, nil
}

func (t T) path() string {
	return t.GetString("path")
}
