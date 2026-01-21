//go:build linux || solaris

package poolzpool

import (
	"context"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/zfs"
)

type (
	T struct {
		pool.T
	}
)

var (
	drvID = driver.NewID(driver.GroupPool, "zpool")
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
	return t.poolName()
}

func (t T) poolName() string {
	return t.GetString("name")
}

func (t T) Capabilities() pool.Capabilities {
	return pool.Capabilities{
		pool.CapBlk,
		pool.CapFile,
		pool.CapROO,
		pool.CapROX,
		pool.CapRWO,
		pool.CapRWX,
		pool.CapSnap,
	}
}

func (t T) Usage(ctx context.Context) (pool.Usage, error) {
	poolName := t.poolName()
	zpool := zfs.Pool{Name: poolName}
	e, err := zpool.Usage(ctx)
	if err != nil {
		return pool.Usage{}, err
	}
	var size, free, used int64
	if e.Size > 0 {
		size = e.Size
		free = e.Free
		used = e.Alloc
	}
	usage := pool.Usage{
		Size: size,
		Free: free,
		Used: used,
	}
	return usage, nil
}

func (t *T) Translate(name string, size int64, shared bool) ([]string, error) {
	poolName := t.poolName()
	mnt := pool.MountPointFromName(name)
	data := []string{
		"fs#0.type=zfs",
		"fs#0.dev=" + poolName + "/" + name,
		"fs#0.mnt=" + mnt,
		"fs#0.size=" + sizeconv.ExactBSizeCompact(float64(size)),
	}
	if mkfsOpt := t.GetString("mkfs_opt"); mkfsOpt != "" {
		data = append(data, "fs#0.mkfs_opt="+mkfsOpt)
	}
	if mntOpt := t.GetString("mnt_opt"); mntOpt != "" {
		data = append(data, "fs#0.mnt_opt="+mntOpt)
	}
	return data, nil
}

func (t *T) BlkTranslate(name string, size int64, shared bool) ([]string, error) {
	poolName := t.poolName()
	data := []string{
		"disk#0.type=zvol",
		"disk#0.dev=" + poolName + "/" + name,
		"disk#0.size=" + sizeconv.ExactBSizeCompact(float64(size)),
	}
	if mkblkOpt := t.GetString("create_options"); mkblkOpt != "" {
		data = append(data, "disk#0.create_options="+mkblkOpt)
	}
	return data, nil
}
