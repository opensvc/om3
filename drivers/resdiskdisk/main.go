package resdiskdisk

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/object"
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
		DiskID    string `json:"disk_id"`
		Name      string `json:"name"`
		Pool      string `json:"pool"`
		Array     string `json:"array"`
		DiskGroup string `json:"diskgroup"`
		SLO       string `json:"slo"`
		Size      *int64 `json:"size"`
		Nodes     []string
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
	} else {
		t.createDisk()
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

func (t T) diskIDKey() key.T {
	k := key.T{
		Section: t.RID(),
		Option:  "disk_id",
	}
	if !t.Shared {
		k.Option += "@" + hostname.Hostname()
	}
	return k
}
func (t T) pooler() (pool.DiskCreator, error) {
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
	creator, ok := p.(pool.DiskCreator)
	if !ok {
		return nil, errors.Errorf("the pool %s driver does not support disk creation", l.Name)
	}
	return creator, nil
}

func (t T) diskName(p pool.DiskCreator) string {
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

func (t T) createDisk() error {
	p, err := t.pooler()
	if err != nil {
		return err
	}
	if t.Size == nil {
		return errors.Errorf("the size keyword is required for disk provisioning")
	}
	diskIDKey := t.diskIDKey()
	diskMapToNodes := t.diskMapToNodes()
	diskName := t.diskName(p)
	size := any(*t.Size).(float64)
	if err := p.CreateDisk(diskName, size, diskMapToNodes); err != nil {
		t.Log().Error().Err(err).Str("name", diskName).Float64("size", size).Strs("nodes", diskMapToNodes).Stringer("disk_id_kw", diskIDKey).Msg("create disk")
	} else {
		t.Log().Info().Str("name", diskName).Float64("size", size).Strs("nodes", diskMapToNodes).Stringer("disk_id_kw", diskIDKey).Msg("create disk")
	}
	return nil
	/*
	   for line in format_str_flat_json(result).splitlines():
	       self.log.info(line)
	   changes = []
	   if "disk_ids" in result:
	       for node, disk_id in result["disk_ids"].items():
	           changes.append("%s.disk_id@%s=%s" % (self.rid, node, disk_id))
	   elif "disk_id" in result:
	       disk_id = result["disk_id"]
	       changes.append("%s.%s=%s" % (self.rid, disk_id_kw, disk_id))
	   else:
	       raise ex.Error("no disk id found in result")
	   self.log.info("changes: %s", changes)
	   self.svc.set_multi(changes, validation=False)
	   self.log.info("changes applied")
	   self.disk_id = self.oget("disk_id")
	   self.log.info("disk %s provisioned" % result["disk_id"])
	*/
}
