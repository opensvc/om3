package object

import (
	"github.com/pkg/errors"
	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/util/disks"
	"github.com/opensvc/om3/util/hostname"
)

func allObjectsDeviceClaims() (disks.ObjectsDeviceClaims, error) {
	claims := disks.NewObjectsDeviceClaims()
	paths, err := path.List()
	if err != nil {
		return claims, err
	}
	objs, err := NewList(paths.Filter("*/svc/*").Merge(paths.Filter("*/vol/*")), WithVolatile(true))
	if err != nil {
		return claims, err
	}
	claims.AddObjects(objs...)
	return claims, err
}

func (t Node) PushDisks() (disks.Disks, error) {
	claims, err := allObjectsDeviceClaims()
	if err != nil {
		return nil, err
	}
	t.Log().Debug().Interface("claims", claims).Msg("PushDisks")
	l, err := disks.GetDisks(claims)
	if err != nil {
		return l, err
	}
	if err := t.pushDisks(l); err != nil {
		return l, err
	}
	return l, nil
}

func pushSvcDisks(client *collector.Client, data disks.Disks) error {
	nodename := hostname.Hostname()
	diskAsList := func(d disks.Disk, r disks.Region) []interface{} {
		return []interface{}{
			d.ID,
			r.Object,
			r.Size / 1024 / 1024,
			r.Size / 1024 / 1024,
			d.Vendor,
			d.Model,
			r.Group,
			nodename,
			0,
		}
	}
	disksAsList := func(t disks.Disks) [][]interface{} {
		l := make([][]interface{}, 0)
		for _, disk := range t {
			for _, region := range disk.Regions {
				l = append(l, diskAsList(disk, region))
			}
		}
		return l
	}
	vars := []string{
		"disk_id",
		"disk_svcname",
		"disk_size",
		"disk_used",
		"disk_vendor",
		"disk_model",
		"disk_dg",
		"disk_nodename",
		"disk_region",
	}
	if response, err := client.Call("register_disks", vars, disksAsList(data)); err != nil {
		return err
	} else if response.Error != nil {
		return errors.Errorf("rpc: %s %s", response.Error.Message, response.Error.Data)
	}

	return nil
}

func pushDiskInfo(client *collector.Client, data disks.Disks) error {
	vars := []string{
		"disk_id",
		"disk_arrayid",
		"disk_devid",
		"disk_size",
		"disk_raid",
		"disk_group",
	}
	vals := [][]string{}
	if response, err := client.Call("register_diskinfo", vars, vals); err != nil {
		return err
	} else if response.Error != nil {
		return errors.Errorf("rpc: %s %s", response.Error.Message, response.Error.Data)
	}
	return nil
}

func (t Node) pushDisks(data disks.Disks) error {
	client, err := t.CollectorFeedClient()
	if err != nil {
		return err
	}
	if err := pushSvcDisks(client, data); err != nil {
		return err
	}
	if err := pushDiskInfo(client, data); err != nil {
		return err
	}
	return nil
}
