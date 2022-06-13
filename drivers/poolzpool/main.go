//go:build linux || solaris

package poolzpool

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/pool"
	"opensvc.com/opensvc/util/sizeconv"
	"opensvc.com/opensvc/util/zfs"
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

func (t T) Capabilities() []string {
	return []string{"rox", "rwx", "roo", "rwo", "snap", "blk"}
}

func (t T) Usage() (pool.StatusUsage, error) {
	poolName := t.poolName()
	zpool := zfs.Pool{Name: poolName}
	e, err := zpool.Usage()
	if err != nil {
		return pool.StatusUsage{}, err
	}
	var size, free, used int64
	if e.Size > 0 {
		size = e.Size / 1024
		free = e.Free / 1024
		used = e.Alloc / 1024
	}
	usage := pool.StatusUsage{
		Size: float64(size),
		Free: float64(free),
		Used: float64(used),
	}
	return usage, nil
}

func (t *T) Translate(name string, size float64, shared bool) []string {
	poolName := t.poolName()
	mnt := pool.MountPointFromName(name)
	data := []string{
		"fs#0.type=zfs",
		"fs#0.dev=" + poolName + "/" + name,
		"fs#0.mnt=" + mnt,
		"fs#0.size=" + sizeconv.ExactBSizeCompact(size),
	}
	if mkfsOpt := t.GetString("mkfs_opt"); mkfsOpt != "" {
		data = append(data, "fs#0.mkfs_opt="+mkfsOpt)
	}
	if mntOpt := t.GetString("mnt_opt"); mntOpt != "" {
		data = append(data, "fs#0.mnt_opt="+mntOpt)
	}
	return data
}

func (t *T) BlkTranslate(name string, size float64, shared bool) []string {
	poolName := t.poolName()
	data := []string{
		"disk#0.type=zvol",
		"fs#0.dev=" + poolName + "/" + name,
		"disk#0.size=" + sizeconv.ExactBSizeCompact(size),
	}
	if mkblkOpt := t.GetString("create_options"); mkblkOpt != "" {
		data = append(data, "fs#0.mkblk_opt="+mkblkOpt)
	}
	return data
}
