package resdiskdisk

import (
	"context"
	"fmt"
	"strings"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/rs/zerolog"
)

type (
	T struct {
		resdisk.T
		DiskID    string   `json:"disk_id"`
		Name      string   `json:"name"`
		Pool      string   `json:"pool"`
		Array     string   `json:"array"`
		DiskGroup string   `json:"diskgroup"`
		SLO       string   `json:"slo"`
		Size      *int64   `json:"size"`
		Nodes     []string `json:"-"`
		Path      path.T   `json:"-"`
	}
	forceMode int
)

const (
	enforce forceMode = iota
	preserve
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t T) Start(ctx context.Context) error {
	return nil
}

func (t T) Stop(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.FromBool(t.DiskID != ""), nil
}

func (t T) Label() string {
	return t.DiskID
}

func (t T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{}
	return m, nil
}

func (t T) UnprovisionLeaded(ctx context.Context) error {
	return t.unconfigure()
}

func (t T) ProvisionLeaded(ctx context.Context) error {
	return t.configure(preserve)
}

func (t *T) ProvisionLeader(ctx context.Context) error {
	var (
		disks []pool.Disk
		err   error
	)
	if t.DiskID != "" {
		t.Log().Info().Msg("skip disk creation: the disk_id keyword is already set")
		return t.configure(preserve)
	}
	if disks, err = t.createDisk(); err != nil {
		return err
	}
	if err := t.setDiskIDKeywords(ctx, disks); err != nil {
		return err
	}
	return t.configure(enforce)
}

func (t *T) UnprovisionLeader(ctx context.Context) error {
	if t.DiskID == "" {
		t.Log().Info().Msg("skip disk deletion: the disk_id keyword is not set")
		return nil
	}
	if err := t.unconfigure(); err != nil {
		return err
	}
	if _, err := t.deleteDisk(); err != nil {
		return err
	}
	if err := t.unsetDiskIDKeywords(ctx); err != nil {
		return err
	}
	return nil
}

func (t T) ReservableDevices() device.L {
	return t.ExposedDevices()
}

func (t T) ClaimedDevices() device.L {
	return t.ExposedDevices()
}

func (t T) diskIDKey(node string) key.T {
	k := key.T{
		Section: t.RID(),
		Option:  "disk_id",
	}
	if !t.Shared {
		k.Option += "@" + node
	}
	return k
}
func (t T) pooler() (pool.ArrayPooler, error) {
	node, err := object.NewNode()
	if err != nil {
		return nil, err
	}
	l := pool.NewLookup(node)
	l.Name = t.Pool
	p, err := l.Do()
	if err != nil {
		return nil, err
	}
	if ap, ok := p.(pool.ArrayPooler); !ok {
		return nil, fmt.Errorf("pool %s is not backed by a storage array", p.Name())
	} else {
		return ap, nil
	}
}

func (t T) diskName(p pool.Pooler) string {
	if t.Shared {
		return t.Name
	} else {
		sep := p.Separator()
		return strings.Join([]string{t.Name, hostname.Hostname()}, sep)
	}
}

func (t T) diskMapToNodes() []string {
	if t.Shared {
		return t.Nodes
	} else {
		return []string{hostname.Hostname()}
	}
}

func (t T) deleteDisk() ([]pool.Disk, error) {
	p, err := t.pooler()
	if err != nil {
		return []pool.Disk{}, err
	}
	diskName := t.diskName(p)
	disks, err := p.DeleteDisk(diskName)
	var ev *zerolog.Event
	if err != nil {
		ev = t.Log().Error().Err(err)
	} else {
		ev = t.Log().Info()
	}
	ev.Str("name", diskName).
		Interface("result", disks).
		Msg("delete disk")
	return disks, nil
}

func (t T) createDisk() ([]pool.Disk, error) {
	p, err := t.pooler()
	if err != nil {
		return []pool.Disk{}, err
	}
	if t.Size == nil {
		return []pool.Disk{}, fmt.Errorf("the size keyword is required for disk provisioning")
	}
	diskName := t.diskName(p)
	size := float64(*t.Size)
	nodes := t.diskMapToNodes()
	paths, err := pool.GetMapping(p, nodes)
	if err != nil {
		return []pool.Disk{}, err
	}
	disks, err := p.CreateDisk(diskName, size, paths)
	if err != nil {
		t.Log().Error().Err(err).Str("name", diskName).Interface("disks", disks).Msg("create disk")
	} else {
		t.Log().Info().Str("name", diskName).Interface("disks", disks).Msg("create disk")
	}
	return disks, err
}

func (t *T) unsetDiskIDKeywords(ctx context.Context) error {
	obj, err := object.NewConfigurer(t.Path)
	if err != nil {
		return err
	}
	section := t.RID()
	options := obj.Config().Keys(section)
	keys := make([]key.T, 0)
	save := make([]keyop.T, 0)
	for _, option := range options {
		switch {
		case option == "disk_id":
			// ok
		case strings.HasPrefix(option, "disk_id@"):
			// ok
		default:
			// not ok
			continue
		}
		k := key.T{section, option}
		keys = append(keys, k)
		save = append(save, keyop.T{
			Key:   k,
			Op:    keyop.Equal,
			Value: obj.Config().GetString(k),
		})
	}
	t.Log().Info().Msgf("unset %s", save)
	return obj.Unset(ctx, keys...)
}

func (t *T) setDiskIDKeywords(ctx context.Context, disks []pool.Disk) error {
	obj, err := object.NewConfigurer(t.Path)
	if err != nil {
		return err
	}
	nodesInfo, err := nodesinfo.Get()
	if err != nil {
		return err
	}
	done := map[string]any{}
	ops := keyop.L{}
	for _, disk := range disks {
		if disk.ID == "" {
			return fmt.Errorf("created disk has no id: %v", disk)
		}
		nodes := nodesInfo.GetNodesWithAnyPaths(disk.Paths)
		for _, node := range nodes {
			if _, ok := done[node]; ok {
				continue
			}
			op := keyop.T{
				Key:   t.diskIDKey(node),
				Op:    keyop.Set,
				Value: disk.ID,
			}
			ops = append(ops, op)
			done[node] = nil
		}
	}
	t.Log().Info().Msgf("set %s", ops)
	if err := obj.Set(ctx, ops...); err != nil {
		return err
	}

	// Set our local node DiskID resource property, for use by T.configure()
	t.DiskID = obj.Config().GetString(key.T{t.RID(), "disk_id"})

	return nil
}
