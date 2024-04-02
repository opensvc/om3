//go:build linux || solaris

package poolpure

import (
	"errors"
	"fmt"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/drivers/arraypure"
	"github.com/opensvc/om3/util/san"
	"github.com/opensvc/om3/util/sizeconv"
)

type (
	T struct {
		pool.T
	}
)

var (
	drvID = driver.NewID(driver.GroupPool, "pure")
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
	s := t.pod()
	if s == "" {
		s = t.volumeGroup()
	}
	return fmt.Sprintf("array://%s/%s", t.arrayName(), s)
}

func (t T) pod() string {
	return t.GetString("pod")
}

func (t T) volumeGroup() string {
	return t.GetString("volume_group")
}

func (t T) deleteNow() bool {
	return t.GetBool("delete_now")
}

func (t T) arrayName() string {
	return t.GetString("array")
}

func (t T) Capabilities() []string {
	return []string{"rox", "rwx", "roo", "rwo", "blk", "fc", "shared"}
}

func (t T) Usage() (pool.Usage, error) {
	usage := pool.Usage{}
	/*
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
	*/
	return usage, nil
}

func (t T) array() *arraypure.Array {
	a := arraypure.New()
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
	ports := make(san.Targets, 0)
	/*
		a := t.array()
		data, err := a.GetISCSITargets()
		if err != nil {
			return nil, err
		}
		for _, d := range data {
			ports = append(ports, san.Target{
				Name: d.Name,
				Type: san.ISCSI,
			})
		}
	*/
	return ports, nil
}

func (t *T) DeleteDisk(name string) ([]pool.Disk, error) {
	disk := pool.Disk{}
	/*
		a := t.array()
		drvName := t.diskgroup() + "/" + name
		drvDisk, err := a.DelDisk(drvName)
		if err != nil {
			return []pool.Disk{}, err
		}
		disk.Driver = drvDisk
		disk.ID = a.DiskID(*drvDisk)
		if paths, err := a.DiskPaths(*drvDisk); err != nil {
			return []pool.Disk{disk}, err
		} else {
			disk.Paths = paths
		}
	*/
	return []pool.Disk{disk}, nil
}

func (t *T) CreateDisk(name string, size int64, paths san.Paths) ([]pool.Disk, error) {
	disk := pool.Disk{}
	if len(paths) == 0 {
		return []pool.Disk{}, errors.New("no mapping in request. cowardly refuse to create a disk that can not be mapped")
	}
	/*
		a := t.array()
		blocksize := fmt.Sprint(*t.blocksize())
		sparse := t.sparse()
		insecureTPC := t.insecureTPC()
		drvSize := sizeconv.ExactBSizeCompact(float64(size))
		drvName := t.diskgroup() + "/" + name
		mapping := paths.Mapping()

		drvDisk, err := a.AddDisk(drvName, drvSize, blocksize, sparse, insecureTPC, mapping, nil)
		if err != nil {
			return []pool.Disk{}, err
		}
		disk.Driver = drvDisk
		disk.ID = a.DiskID(*drvDisk)
		if paths, err := a.DiskPaths(*drvDisk); err != nil {
			return []pool.Disk{disk}, err
		} else {
			disk.Paths = paths
		}
	*/
	return []pool.Disk{disk}, nil
}
