package resdiskdisk

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/nodesinfo"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/pool"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
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

func (t T) Info() map[string]string {
	m := make(map[string]string)
	return m
}

func (t T) ProvisionLeader(ctx context.Context) error {
	if v, err := t.Provisioned(); err != nil {
		return err
	} else if v == provisioned.True {
		t.Log().Info().Msg("skip disk creation: the disk_id keyword is already set")
		return t.configure(preserve)
	} else if err := t.createDisk(ctx); err != nil {
		return err
	} else {
		return t.configure(enforce)
	}
}

func (t T) UnprovisionLeader(ctx context.Context) error {
	return nil
}

func (t T) ClaimedDevices() []*device.T {
	return t.ExposedDevices()
}

// configure is os specific
func (t T) configure(force forceMode) error {
	return nil
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
		return nil, errors.Errorf("pool %s is not backed by a storage array", p.Name())
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

func (t T) createDisk(ctx context.Context) error {
	p, err := t.pooler()
	if err != nil {
		return err
	}
	if t.Size == nil {
		return errors.Errorf("the size keyword is required for disk provisioning")
	}
	nodesInfo, err := nodesinfo.Get()
	if err != nil {
		return err
	}
	obj, err := object.NewConfigurer(t.Path)
	if err != nil {
		return err
	}
	diskName := t.diskName(p)
	size := float64(*t.Size)
	nodes := t.diskMapToNodes()
	paths, err := pool.GetMapping(p, nodes)
	if err != nil {
		return err
	}
	createDiskResult, err := p.CreateDisk(pool.CreateDiskRequest{
		Name:  diskName,
		Size:  size,
		Paths: paths,
	})
	var ev *zerolog.Event
	if err != nil {
		ev = t.Log().Error().Err(err)
	} else {
		ev = t.Log().Info()
	}
	ev.Str("name", diskName).
		Interface("result", createDiskResult).
		Msg("create disk")
	if err != nil {
		return err
	}
	ops := keyop.L{}
	for _, disk := range createDiskResult.Disks {
		if disk.ID == "" {
			return errors.Errorf("created disk has no id: %v", disk)
		}
		nodes := nodesInfo.GetNodesWithAnyPaths(disk.Paths)
		for _, node := range nodes {
			op := keyop.T{
				Key:   t.diskIDKey(node),
				Op:    keyop.Set,
				Value: disk.ID,
			}
			ops = append(ops, op)
		}
	}
	if err = obj.Set(ctx, ops...); err != nil {
		return err
	}
	return nil
}
