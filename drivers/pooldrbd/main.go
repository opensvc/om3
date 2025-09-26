//go:build linux

package pooldrbd

import (
	"fmt"
	"path/filepath"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/df"
	"github.com/opensvc/om3/util/lvm2"
	"github.com/opensvc/om3/util/sizeconv"
	"github.com/opensvc/om3/util/zfs"
)

type (
	T struct {
		pool.T
	}
)

var (
	drvID = driver.NewID(driver.GroupPool, "drbd")
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

func (t T) Capabilities() []string {
	return []string{"rox", "rwx", "roo", "rwo", "snap", "blk", "shared"}
}

func (t T) vg() string {
	return t.GetString("vg")
}

func (t T) zpool() string {
	return t.GetString("zpool")
}

func (t T) maxPeers() string {
	return t.GetString("max_peers")
}

func (t T) network() string {
	return t.GetString("network")
}

func (t *T) template() string {
	return t.GetString("template")
}

func (t T) addrs() map[string]string {
	m := make(map[string]string)
	for _, nodename := range cluster.ConfigData.Get().Nodes {
		s := t.GetStringAs("addr", nodename)
		if s != "" {
			m[nodename] = s
		}
	}
	return m
}

func (t T) path() string {
	if p := t.GetString("path"); p != "" {
		return p
	}
	return filepath.Join(rawconfig.Paths.Var, "pool", t.Name())
}

func (t T) Head() string {
	if vg := t.vg(); vg != "" {
		return vg
	} else if zpool := t.zpool(); zpool != "" {
		return zpool
	} else {
		return t.path()
	}
}

func (t T) Usage() (pool.Usage, error) {
	if t.vg() != "" {
		return t.usageVG()
	} else if t.zpool() != "" {
		return t.usageZpool()
	} else {
		return t.usageFile()
	}
}

func (t T) usageVG() (pool.Usage, error) {
	vg := lvm2.NewVG(t.vg())
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

func (t T) usageZpool() (pool.Usage, error) {
	poolName := t.zpool()
	zpool := zfs.Pool{Name: poolName}
	e, err := zpool.Usage()
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

func (t T) usageFile() (pool.Usage, error) {
	entries, err := df.ContainingMountUsage(t.path())
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

func (t *T) blkTranslateFile(name string, size int64, shared bool) (string, []string, error) {
	p := fmt.Sprintf("%s/%s.img", t.path(), name)
	data := []string{
		"fs#0.type=flag",
		"disk#1.type=loop",
		"disk#1.name=" + name,
		"disk#1.size=" + sizeconv.ExactBSizeCompact(float64(size)),
		"disk#1.file=" + p,
		"disk#1.standby=true",
		"disk#2.type=vg",
		"disk#2.name=" + name,
		"disk#2.pvs=" + p,
		"disk#2.standby=true",
		"disk#3.type=lv",
		"disk#3.name=lv",
		"disk#3.vg=" + name,
		"disk#3.size=100%FREE",
		"disk#3.standby=true",
		"disk#4.type=drbd",
		"disk#4.res=" + name,
		"disk#4.disk=/dev/" + name + "/lv",
		"disk#4.standby=true",
	}
	if opts := t.MkblkOptions(); opts != "" {
		data = append(data, "disk#3.create_options="+opts)
	}
	data = append(data, t.commonDrbdKeywords("disk#4")...)
	return "disk#4", data, nil
}

func (t *T) blkTranslateVG(name string, size int64, shared bool) (string, []string, error) {
	data := []string{
		"fs#0.type=flag",
		"disk#1.type=lv",
		"disk#1.name=" + name,
		"disk#1.vg=" + t.vg(),
		"disk#1.size=" + sizeconv.ExactBSizeCompact(float64(size)),
		"disk#1.standby=true",
		"disk#2.type=drbd",
		"disk#2.res=" + name,
		"disk#2.disk=/dev/" + t.vg() + "/" + name,
		"disk#2.standby=true",
	}
	if opts := t.MkblkOptions(); opts != "" {
		data = append(data, "disk#1.create_options="+opts)
	}
	data = append(data, t.commonDrbdKeywords("disk#2")...)
	return "disk#2", data, nil
}

func (t *T) blkTranslateZpool(name string, size int64, shared bool) (string, []string, error) {
	data := []string{
		"fs#0.type=flag",
		"disk#1.type=zvol",
		"disk#1.dev=" + t.zpool() + "/" + name,
		"disk#1.size=" + sizeconv.ExactBSizeCompact(float64(size)),
		"disk#1.standby=true",
		"disk#2.type=drbd",
		"disk#2.res=" + name,
		"disk#2.disk=/dev/" + t.zpool() + "/" + name,
		"disk#2.standby=true",
	}
	if opts := t.MkblkOptions(); opts != "" {
		data = append(data, "disk#1.create_options="+opts)
	}
	data = append(data, t.commonDrbdKeywords("disk#2")...)
	return "disk#2", data, nil
}

func (t *T) commonDrbdKeywords(rid string) (l []string) {
	if s := t.maxPeers(); s != "" {
		l = append(l, rid+".max_peers="+s)
	}
	if s := t.network(); s != "" {
		l = append(l, rid+".network="+s)
	}
	if s := t.template(); s != "" {
		l = append(l, rid+".template="+s)
	}
	for nodename, addr := range t.addrs() {
		l = append(l, rid+".addr@"+nodename+"="+addr)
	}
	return
}

func (t *T) commonTranslate(name string, size int64, shared bool) (string, []string, error) {
	if t.vg() != "" {
		return t.blkTranslateVG(name, size, shared)
	} else if t.zpool() != "" {
		return t.blkTranslateZpool(name, size, shared)
	} else {
		return t.blkTranslateFile(name, size, shared)
	}
}

func (t *T) BlkTranslate(name string, size int64, shared bool) ([]string, error) {
	_, kws, err := t.commonTranslate(name, size, shared)
	return kws, err
}

func (t *T) Translate(name string, size int64, shared bool) ([]string, error) {
	rid, kws, err := t.commonTranslate(name, size, shared)
	if err != nil {
		return nil, err
	}
	kws = append(kws, t.AddFS(name, shared, 1, 0, rid)...)
	return kws, nil
}
