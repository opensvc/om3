package poolsymmetrix

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/drivers/arraysymmetrix"
	"github.com/opensvc/om3/v3/util/hostname"
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
	drvID = driver.NewID(driver.GroupPool, "symmetrix")
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
	s := ""
	if sid := t.sid(); sid != "" {
		s += fmt.Sprintf("array://%s", sid)
	} else {
		s += fmt.Sprintf("array://%s", t.arrayName())
	}
	if srp := t.srp(); srp != "" {
		s += fmt.Sprintf("/%s", srp)
	}
	return s
}

func (t T) labelPrefix() string {
	return t.GetString("label_prefix")
}

func (t T) slo() string {
	return t.GetString("slo")
}

func (t T) srp() string {
	return t.GetString("srp")
}

func (t T) srdf() bool {
	return t.GetBool("srdf")
}

func (t T) sid() string {
	return t.GetString("sid")
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

func (t T) remoteArrayName() string {
	localArrayName := t.arrayName()
	nodenames, err := t.array().Config().NodeReferrer.Nodes()
	if err != nil {
		return ""
	}
	for _, nodename := range nodenames {
		if nodename == hostname.Hostname() {
			continue
		}
		remoteArrayName := t.GetStringAs("array", nodename)
		if remoteArrayName != localArrayName {
			return remoteArrayName
		}
	}
	return ""
}

func (t T) arrayNodes(nodenames []string) (map[string][]string, error) {
	m := make(map[string][]string)
	if len(nodenames) == 0 {
		if l, err := t.array().Config().NodeReferrer.Nodes(); err != nil {
			return m, err
		} else {
			nodenames = l
		}
	}
	for _, nodename := range nodenames {
		arrayName := t.GetStringAs("array", nodename)
		if arrayName != "" {
			if l, ok := m[nodename]; ok {
				l = append(l, arrayName)
				m[nodename] = l
			} else {
				l = []string{arrayName}
				m[nodename] = l
			}
		}

	}
	return m, nil
}

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

func (t T) getSRP(ctx context.Context) (arraysymmetrix.SRP, error) {
	srps, err := t.array().SymCfgSRPList(ctx)
	if err != nil {
		return arraysymmetrix.SRP{}, err
	}
	srpName := t.srp()
	for _, srp := range srps {
		if srp.SRPInfo.Name == srpName {
			return srp, nil
		}
	}
	return arraysymmetrix.SRP{}, os.ErrNotExist
}

func (t T) Usage(ctx context.Context) (pool.Usage, error) {
	usage := pool.Usage{
		Shared: true,
	}

	srp, err := t.getSRP(ctx)
	if err != nil {
		return usage, err
	}

	usage.Size = int64(srp.SRPInfo.UsableCapacityGigabytes)
	usage.Used = int64(srp.SRPInfo.FreeCapacityGigabytes)
	usage.Free = int64(srp.SRPInfo.UsedCapacityGigabytes)
	return usage, nil
}

func (t T) array() *arraysymmetrix.Array {
	a := arraysymmetrix.New()
	a.SetName(t.arrayName())
	a.SetConfig(t.Config().(*xconfig.T))
	return a
}

func (t T) remoteArray() *arraysymmetrix.Array {
	a := arraysymmetrix.New()
	a.SetName(t.remoteArrayName())
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
	data, err := t.array().SymCfgDirectorList(ctx, "all")
	if err != nil {
		return nil, err
	}
	for _, d := range data {
		for _, port := range d.Ports {
			if port.PortInfo.PortWWN != "" {
				ports = append(ports, san.Target{
					Name: port.PortInfo.PortWWN,
					Type: san.FC,
				})
			}
		}
	}
	return ports, nil
}

func (t *T) DeleteDisk(ctx context.Context, name, wwid string) ([]pool.Disk, error) {
	poolDisk := pool.Disk{}
	a := t.array()
	if len(wwid) != 32 {
		return nil, fmt.Errorf("wwid %s is not 32 characters long", wwid)
	}
	arrayDisk, err := a.DelDisk(ctx, arraysymmetrix.OptDelDisk{
		Dev: wwid,
	})
	if err != nil {
		return []pool.Disk{}, err
	}
	poolDisk.Driver = arrayDisk.DriverData
	poolDisk.ID = arrayDisk.DiskID
	return []pool.Disk{poolDisk}, nil
}

func (t *T) CreateDisk(ctx context.Context, name string, size int64, nodenames []string) ([]pool.Disk, error) {
	if t.srdf() {
		return t.CreateDiskSRDF(ctx, name, size, nodenames)
	} else {
		return t.CreateDiskSimple(ctx, name, size, nodenames)
	}
}

func (t *T) CreateDiskSRDF(ctx context.Context, name string, size int64, nodenames []string) ([]pool.Disk, error) {
	arrayNodes, err := t.arrayNodes(nodenames)
	if err != nil {
		return []pool.Disk{}, err
	}

	r1Nodes := arrayNodes[t.arrayName()]
	r2Nodes := arrayNodes[t.remoteArrayName()]

	r1PoolDisks, err := t.CreateDiskSimple(ctx, name, size, r1Nodes)
	if err != nil {
		return []pool.Disk{}, err
	}

	r2PoolDisks, err := t.MapDisk(ctx, name, r2Nodes)
	if err != nil {
		return []pool.Disk{}, err
	}

	return append(r1PoolDisks, r2PoolDisks...), nil
}

func (t *T) MapDisk(ctx context.Context, devId string, nodenames []string) ([]pool.Disk, error) {
	poolDisk := pool.Disk{}
	paths, err := pool.GetPaths(ctx, t, nodenames, "fc")
	mappings, err := pool.GetMappings(ctx, t, nodenames, "fc")
	if err != nil {
		return []pool.Disk{}, err
	}
	arrayDisk, err := t.remoteArray().MapDisk(ctx, arraysymmetrix.OptMapDisk{
		Dev:      devId,
		SLO:      t.slo(),
		SRP:      t.srp(),
		SID:      t.sid(),
		Mappings: mappings,
	})

	if err != nil {
		return []pool.Disk{}, err
	}
	poolDisk.Driver = arrayDisk.DriverData
	poolDisk.ID = arrayDisk.DiskID
	poolDisk.Paths = paths
	return []pool.Disk{poolDisk}, nil
}

func (t *T) CreateDiskSimple(ctx context.Context, name string, size int64, nodenames []string) ([]pool.Disk, error) {
	poolDisk := pool.Disk{}
	paths, err := pool.GetPaths(ctx, t, nodenames, "fc")
	mappings, err := pool.GetMappings(ctx, t, nodenames, "fc")
	if err != nil {
		return []pool.Disk{}, err
	}

	if len(paths) == 0 {
		return []pool.Disk{}, errors.New("no mapping in request. cowardly refuse to create a disk that can not be mapped")
	}
	drvSize := sizeconv.ExactBSizeCompact(float64(size))
	arrayDisk, err := t.array().AddDisk(ctx, arraysymmetrix.OptAddDisk{
		Name:     name,
		Size:     drvSize,
		SID:      t.sid(),
		SRP:      t.srp(),
		SLO:      t.slo(),
		SRDF:     t.srdf(),
		Mappings: mappings,
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
