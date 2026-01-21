//go:build linux || solaris

package poolpure

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/drivers/arraypure"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/san"
	"github.com/opensvc/om3/v3/util/sizeconv"
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

func (t T) Capabilities() pool.Capabilities {
	return pool.Capabilities{
		pool.CapBlk,
		pool.CapFile,
		pool.CapMove,
		pool.CapROO,
		pool.CapROX,
		pool.CapRWO,
		pool.CapRWX,
		pool.CapShared,
	}
}

func (t T) Usage(ctx context.Context) (pool.Usage, error) {
	usage := pool.Usage{
		Shared: true,
	}
	a := t.array()
	data, err := a.GetArrays(ctx, arraypure.OptGetItems{})
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

func (t *T) GetTargets(ctx context.Context) (san.Targets, error) {
	ports := make(san.Targets, 0)
	a := t.array()
	opt := arraypure.OptGetItems{
		Filter: "services='scsi-fc' and enabled='true'",
	}
	data, err := a.GetNetworkInterfaces(ctx, opt)
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

func (t *T) DeleteDisk(ctx context.Context, name, wwid string) ([]pool.Disk, error) {
	if len(wwid) != 32 {
		return nil, fmt.Errorf("delete disk: can not fetch serial from wwid: %s", wwid)
	}
	serial := wwid[8:]
	poolDisk := pool.Disk{}
	a := t.array()
	arrayDisk, err := a.DelDisk(ctx, arraypure.OptDelDisk{
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

func (t *T) CreateDisk(ctx context.Context, name string, size int64, nodenames []string) ([]pool.Disk, error) {
	poolDisk := pool.Disk{}
	paths, err := pool.GetPaths(ctx, t, nodenames, san.FC)
	if err != nil {
		return []pool.Disk{}, err
	}
	if len(paths) == 0 {
		return []pool.Disk{}, errors.New("no mapping in request. cowardly refuse to create a disk that can not be mapped")
	}
	a := t.array()
	drvSize := sizeconv.ExactBSizeCompact(float64(size))
	pod := t.pod()
	vg := t.volumeGroup()
	if pod != "" {
		name = pod + "::" + name
	} else if vg != "" {
		name = vg + "/" + name
	}
	arrayDisk, err := a.AddDisk(ctx, arraypure.OptAddDisk{
		Name:     name,
		Size:     drvSize,
		Mappings: paths.MappingList(),
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
		k := key.T{
			Section: "cluster",
			Option:  "id",
		}
		clusterID := t.Config().GetString(k)
		s += strings.SplitN(clusterID, "-", 2)[0] + "-"

	}
	suffix := vol.Config().GetString(key.T{
		Section: "DEFAULT",
		Option:  "id",
	})
	s += strings.SplitN(suffix, "-", 2)[0]
	return s
}
