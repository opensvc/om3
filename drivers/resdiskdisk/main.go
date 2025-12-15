package resdiskdisk

import (
	"context"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodesinfo"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	T struct {
		resdisk.T
		DiskID    string      `json:"disk_id"`
		Name      string      `json:"name"`
		Pool      string      `json:"pool"`
		Array     string      `json:"array"`
		DiskGroup string      `json:"diskgroup"`
		SLO       string      `json:"slo"`
		Size      *int64      `json:"size"`
		Nodes     []string    `json:"-"`
		Path      naming.Path `json:"-"`
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

func (t *T) Start(ctx context.Context) error {
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.FromBool(t.DiskID != ""), nil
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.DiskID
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{}
	return m, nil
}

func (t *T) UnprovisionAsFollower(ctx context.Context) error {
	return t.unconfigure(ctx)
}

func (t *T) ProvisionAsFollower(ctx context.Context) error {
	return t.configure(ctx, preserve)
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	var (
		disks []pool.Disk
		err   error
	)
	if t.DiskID != "" {
		t.Log().Infof("skip disk creation: the disk_id keyword is already set")
		return t.configure(ctx, preserve)
	}
	if disks, err = t.createDisk(ctx); err != nil {
		return err
	}
	if err := t.setDiskIDKeywords(ctx, disks); err != nil {
		return err
	}
	return t.configure(ctx, enforce)
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	if t.DiskID == "" {
		t.Log().Infof("skip disk deletion: the disk_id keyword is not set")
		return nil
	}
	if err := t.unconfigure(ctx); err != nil {
		return err
	}
	if _, err := t.deleteDisk(ctx); err != nil {
		return err
	}
	if err := t.unsetDiskIDKeywords(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) ReservableDevices(ctx context.Context) device.L {
	return t.ExposedDevices(ctx)
}

func (t *T) ClaimedDevices(ctx context.Context) device.L {
	return t.ExposedDevices(ctx)
}

func (t *T) diskIDKey(node string) key.T {
	k := key.T{
		Section: t.RID(),
		Option:  "disk_id",
	}
	if !t.Shared {
		k.Option += "@" + node
	}
	return k
}
func (t *T) pooler(ctx context.Context) (pool.ArrayPooler, error) {
	node, err := object.NewNode()
	if err != nil {
		return nil, err
	}
	l := pool.NewLookup(node)
	l.Name = t.Pool
	p, err := l.Do(ctx)
	if err != nil {
		return nil, err
	}
	if ap, ok := p.(pool.ArrayPooler); !ok {
		return nil, fmt.Errorf("pool %s is not backed by a storage array", p.Name())
	} else {
		return ap, nil
	}
}

func (t *T) diskName(p pool.Pooler) string {
	if t.Shared {
		return t.Name
	} else {
		sep := p.Separator()
		return strings.Join([]string{t.Name, hostname.Hostname()}, sep)
	}
}

func (t *T) diskMapToNodes() []string {
	if t.Shared {
		return t.Nodes
	} else {
		return []string{hostname.Hostname()}
	}
}

func (t *T) deleteDisk(ctx context.Context) ([]pool.Disk, error) {
	p, err := t.pooler(ctx)
	if err != nil {
		return []pool.Disk{}, err
	}
	name := t.diskName(p)
	disks, err := p.DeleteDisk(ctx, name, t.DiskID)
	if err != nil {
		t.Log().Errorf("delete disk %s [%s]: %#v %s", name, t.DiskID, disks, err)
	} else {
		t.Log().Infof("delete disk %s [%s]: %#v", name, t.DiskID, disks)
	}
	return disks, nil
}

func (t *T) createDisk(ctx context.Context) ([]pool.Disk, error) {
	p, err := t.pooler(ctx)
	if err != nil {
		return []pool.Disk{}, err
	}
	if t.Size == nil {
		return []pool.Disk{}, fmt.Errorf("the size keyword is required for disk provisioning")
	}
	diskName := t.diskName(p)
	nodenames := t.diskMapToNodes()
	disks, err := p.CreateDisk(ctx, diskName, *t.Size, nodenames)
	if err != nil {
		t.Log().Errorf("create disk %s: %#v %s", diskName, disks, err)
	} else {
		t.Log().Infof("create disk %s: %#v", diskName, disks)
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
		k := key.T{Section: section, Option: option}
		keys = append(keys, k)
		save = append(save, keyop.T{
			Key:   k,
			Op:    keyop.Equal,
			Value: obj.Config().GetString(k),
		})
	}
	t.Log().Infof("unset %s", save)
	return obj.Unset(ctx, keys...)
}

func (t *T) setDiskIDKeywords(ctx context.Context, disks []pool.Disk) error {
	obj, err := object.NewConfigurer(t.Path)
	if err != nil {
		return err
	}
	nodesInfo, err := nodesinfo.Load()
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
	t.Log().Infof("set %s", ops)
	if err := obj.Set(ctx, ops...); err != nil {
		return err
	}

	// Set our local node DiskID resource property, for use by Path.configure()
	t.DiskID = obj.Config().GetString(key.T{Section: t.RID(), Option: "disk_id"})

	return nil
}
