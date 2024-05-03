//go:build linux || solaris

package poolhoc

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/drivers/arrayhoc"
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
	drvID = driver.NewID(driver.GroupPool, "hoc")
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
	if vsmId := t.vsmId(); vsmId != "" {
		return fmt.Sprintf("array://%s/%s", t.arrayName(), vsmId)
	} else {
		return fmt.Sprintf("array://%s", t.arrayName())
	}
}

func (t T) labelPrefix() string {
	return t.GetString("label_prefix")
}

func (t T) poolId() string {
	return t.GetString("pool_id")
}

func (t T) volumeIdRangeFrom() int {
	return t.GetInt("volume_id_range_from")
}

func (t T) volumeIdRangeTo() int {
	return t.GetInt("volume_id_range_to")
}

func (t T) vsmId() string {
	return t.GetString("vsm_id")
}

func (t T) compression() bool {
	return t.GetBool("compression")
}

func (t T) deduplication() bool {
	return t.GetBool("dedup")
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
	data, err := a.GetStorageSystem()
	if err != nil {
		return usage, err
	}
	usage.Size = data.TotalUsableCapacity
	usage.Used = data.UsedCapacity
	usage.Free = data.AvailableCapacity
	return usage, nil
}

func (t T) array() *arrayhoc.Array {
	a := arrayhoc.New()
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
	opt := arrayhoc.OptGetItems{
		Filter: "type:FIBRE",
	}
	data, err := a.GetStoragePorts(opt)
	if err != nil {
		return nil, err
	}
	for _, d := range data {
		ports = append(ports, san.Target{
			Name: strings.ToLower(d.WWN),
			Type: san.FC,
		})
	}
	return ports, nil
}

func (t *T) DeleteDisk(name, wwid string) ([]pool.Disk, error) {
	if len(wwid) != 32 {
		return nil, fmt.Errorf("delete disk: can not fetch serial from wwid: %s", wwid)
	}
	poolDisk := pool.Disk{}
	a := t.array()
	if len(wwid) != 32 {
		return nil, fmt.Errorf("wwid %s is not 32 characters long")
	}
	devId, err := strconv.ParseInt(wwid[26:], 16, 64)
	if err != nil {
		return nil, err
	}
	arrayDisk, err := a.DelDisk(arrayhoc.OptDelDisk{
		Volume: arrayhoc.OptVolume{
			ID: int(devId),
		},
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
	arrayDisk, err := a.AddDisk(arrayhoc.OptAddDisk{
		Volume: arrayhoc.OptAddVolume{
			Name:                    name,
			Size:                    drvSize,
			PoolId:                  t.poolId(),
			Compression:             t.compression(),
			Deduplication:           t.deduplication(),
			VirtualStorageMachineId: t.vsmId(),
		},
		Mapping: arrayhoc.OptMapping{
			Mappings:          paths.MappingList(),
			VolumeIdRangeFrom: t.volumeIdRangeFrom(),
			VolumeIdRangeTo:   t.volumeIdRangeTo(),
			LUN:               -1,
		},
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
