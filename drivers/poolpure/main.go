//go:build linux || solaris

package poolpure

import (
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/drivers/arraypure"
	"github.com/opensvc/om3/util/key"
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

func (t T) labelPrefix() string {
	return t.GetString("label_prefix")
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
	a := t.array()
	data, err := a.GetArrays(arraypure.OptGetItems{})
	if err != nil {
		return usage, err
	}
	if len(data) == 0 {
		return usage, fmt.Errorf("empty get arrays response")
	}
	space := data[0].Space
	usage.Size = data[0].Capacity
	usage.Used = space.TotalPhysical
	usage.Free = usage.Size - usage.Used
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
	a := t.array()
	opt := arraypure.OptGetItems{
		Filter: "services='scsi-fc' and enabled='true'",
	}
	data, err := a.GetNetworkInterfaces(opt)
	if err != nil {
		return nil, err
	}
	for _, d := range data {
		ports = append(ports, san.Target{
			Name: strings.ToLower(strings.ReplaceAll(*d.FC.WWN, ":", "")),
			Type: san.FC,
		})
	}
	return ports, nil
}

func (t *T) DeleteDisk(name string) ([]pool.Disk, error) {
	if len(name) != 16 {
		return nil, fmt.Errorf("can not fetch serial from disk name to delete: %s", name)
	}
	serial := name[8:]
	poolDisk := pool.Disk{}
	a := t.array()
	arrayDisk, err := a.DelDisk(arraypure.OptDelDisk{
		Volume: arraypure.OptVolume{
			Serial: serial,
		},
		Now: true,
	})
	if err != nil {
		return []pool.Disk{}, err
	}
	poolDisk.Driver = arrayDisk.DriverData
	poolDisk.ID = arrayDisk.DiskID
	return []pool.Disk{poolDisk}, nil
}

func (t *T) CreateDisk(name string, size int64, paths san.Paths) ([]pool.Disk, error) {
	poolDisk := pool.Disk{}
	if len(paths) == 0 {
		return []pool.Disk{}, errors.New("no mapping in request. cowardly refuse to create a disk that can not be mapped")
	}
	a := t.array()
	drvSize := sizeconv.ExactBSizeCompact(float64(size))
	mappings := paths.MappingList()
	pod := t.pod()
	vg := t.volumeGroup()
	if pod != "" {
		name = pod + "::" + name
	} else if vg != "" {
		name = vg + "/" + name
	}
	arrayDisk, err := a.AddDisk(arraypure.OptAddDisk{
		Name:     name,
		Size:     drvSize,
		Mappings: mappings,
		LUN:      -1,
	})
	if err != nil {
		return []pool.Disk{}, err
	}
	poolDisk.Driver = arrayDisk.DriverData
	poolDisk.ID = arrayDisk.DiskID
	poolDisk.Paths = paths
	return []pool.Disk{poolDisk}, nil
}

func (t *T) DiskName(vol pool.Volumer) string {
	var s string
	if labelPrefix := t.labelPrefix(); labelPrefix != "" {
		s += labelPrefix
	} else {
		k := key.T{"cluster", "id"}
		clusterID := t.Config().GetString(k)
		s += strings.SplitN(clusterID, "-", 2)[0] + "-"

	}
	suffix := vol.Config().GetString(key.T{"DEFAULT", "id"})
	s += strings.SplitN(suffix, "-", 2)[0]
	return s
}
