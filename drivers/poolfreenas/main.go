//go:build linux || solaris

package poolfreenas

import (
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/drivers/arrayfreenas"
	"github.com/opensvc/om3/v3/util/san"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	T struct {
		pool.T
	}
)

var (
	drvID = driver.NewID(driver.GroupPool, "truenas")
)

func init() {
	driver.Register(driver.NewID(driver.GroupPool, "freenas"), NewPooler)
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
	return fmt.Sprintf("array://%s/%s", t.arrayName(), t.diskgroup())
}

func (t T) diskgroup() string {
	return t.GetString("diskgroup")
}

func (t T) insecureTPC() bool {
	return t.GetBool("insecureTPC")
}

func (t T) compression() string {
	return t.GetString("compression")
}

func (t T) dedup() string {
	return t.GetString("compression")
}

func (t T) sparse() bool {
	return t.GetBool("sparse")
}

func (t T) blocksize() *int64 {
	return t.GetSize("blocksize")
}

func (t T) arrayName() string {
	return t.GetString("array")
}

func (t T) Capabilities() []string {
	return []string{"rox", "rwx", "roo", "rwo", "blk", "iscsi", "shared"}
}

func (t T) Usage() (pool.Usage, error) {
	usage := pool.Usage{
		Shared: true,
	}
	a := t.array()
	data, err := a.GetDataset(t.diskgroup())
	if err != nil {
		return usage, err
	}
	if i, err := sizeconv.FromSize(data.Used.Rawvalue); err != nil {
		return usage, err
	} else {
		usage.Used = i
	}
	if i, err := sizeconv.FromSize(data.Available.Rawvalue); err != nil {
		return usage, err
	} else {
		usage.Free = i
	}
	usage.Size = usage.Used + usage.Free
	return usage, nil
}

func (t T) array() *arrayfreenas.Array {
	a := arrayfreenas.New()
	a.SetName(t.arrayName())
	a.SetConfig(t.Config().(*xconfig.T))
	return a
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
	data := []string{
		"disk#0.type=disk",
		"disk#0.name=" + name,
		"disk#0.scsireserv=true",
		"shared=" + fmt.Sprint(shared),
		"size=" + sizeconv.ExactBSizeCompact(float64(size)),
	}
	return data, nil
}

func (t *T) GetTargets() (san.Targets, error) {
	a := t.array()
	data, err := a.GetISCSITargets()
	if err != nil {
		return nil, err
	}
	ports := make(san.Targets, 0)
	for _, d := range data {
		ports = append(ports, san.Target{
			Name: d.Name,
			Type: san.ISCSI,
		})
	}
	return ports, nil
}

func (t *T) DeleteDisk(name, wwid string) ([]pool.Disk, error) {
	disk := pool.Disk{}
	a := t.array()
	drvName := t.diskgroup() + "/" + name
	drvDisk, err := a.DelDisk(drvName)
	if err != nil {
		return []pool.Disk{}, err
	}
	disk.Driver = drvDisk
	disk.ID = a.DiskId(*drvDisk)
	if paths, err := a.DiskPaths(*drvDisk); err != nil {
		return []pool.Disk{disk}, err
	} else {
		disk.Paths = paths
	}
	return []pool.Disk{disk}, nil
}

func (t *T) CreateDisk(name string, size int64, nodenames []string) ([]pool.Disk, error) {
	disk := pool.Disk{}
	paths, err := pool.GetPaths(t, nodenames, san.ISCSI)
	if err != nil {
		return []pool.Disk{}, err
	}
	if len(paths) == 0 {
		return []pool.Disk{}, errors.New("no mapping in request. cowardly refuse to create a disk that can not be mapped")
	}
	a := t.array()
	opt := arrayfreenas.AddDiskOptions{
		AddZvolOptions: arrayfreenas.AddZvolOptions{
			Name:          t.diskgroup() + "/" + name,
			Size:          sizeconv.ExactBSizeCompact(float64(size)),
			Blocksize:     fmt.Sprint(*t.blocksize()),
			Sparse:        t.sparse(),
			Compression:   t.compression(),
			Deduplication: t.dedup(),
		},
		InsecureTPC: t.insecureTPC(),
		Mapping:     paths.Mapping(),
		LunId:       nil,
	}
	drvDisk, err := a.AddDisk(opt)
	if err != nil {
		return []pool.Disk{}, err
	}
	disk.Driver = drvDisk
	disk.ID = a.DiskId(*drvDisk)
	if paths, err := a.DiskPaths(*drvDisk); err != nil {
		return []pool.Disk{disk}, err
	} else {
		disk.Paths = paths
	}
	return []pool.Disk{disk}, nil
}
