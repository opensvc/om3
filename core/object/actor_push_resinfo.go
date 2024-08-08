package object

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceselector"
	"github.com/opensvc/om3/util/hostname"
)

// PushResInfo pushes resources information of the local instance of the object
func (t *actor) PushResInfo(ctx context.Context) (resource.Infos, error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.PushResInfo)
	if err := t.validateAction(); err != nil {
		return resource.Infos{}, err
	}
	t.setenv("push resinfo", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return resource.Infos{}, err
	}
	defer unlock()
	return t.lockedPushResInfo(ctx)
}

func (t *actor) lockedPushResInfo(ctx context.Context) (resource.Infos, error) {
	infos := resource.NewInfos(t.Path())
	if more, err := t.masterResInfo(ctx); err != nil {
		return infos, err
	} else {
		infos.Resources = append(infos.Resources, more...)
	}
	if more, err := t.slaveResInfo(ctx); err != nil {
		return infos, err
	} else {
		infos.Resources = append(infos.Resources, more...)
	}
	return infos, t.collectorPushResInfo(infos)
}

func (t *actor) masterResInfo(ctx context.Context) ([]resource.Info, error) {
	l := make([]resource.Info, 0)
	resourceLister := resourceselector.FromContext(ctx, t)
	barrier := actioncontext.To(ctx)
	err := t.ResourceSets().Do(ctx, resourceLister, barrier, "resinfo", func(ctx context.Context, r resource.Driver) error {
		info, err := resource.GetInfo(ctx, r)
		if err != nil {
			return err
		}
		l = append(l, info)
		return nil
	})
	return l, err
}

func (t *actor) slaveResInfo(ctx context.Context) ([]resource.Info, error) {
	return []resource.Info{}, nil
}

func (t *actor) collectorPushResInfo(infos resource.Infos) error {
	svcname := infos.ObjectPath.String()
	nodename := hostname.Hostname()
	topology := t.Topology().String()
	asList := func(infos resource.Infos) [][]string {
		l := make([][]string, 0)
		for _, info := range infos.Resources {
			for _, key := range info.Keys {
				e := []string{
					svcname,
					nodename,
					topology,
					info.RID,
					key.Key,
					key.Value,
				}
				l = append(l, e)
			}
		}
		return l
	}

	vars := []string{
		"res_svcname",
		"res_nodename",
		"topology",
		"rid",
		"res_key",
		"res_value",
	}
	node, err := t.Node()
	if err != nil {
		return err
	}
	client, err := node.CollectorFeedClient()
	if err != nil {
		return err
	}
	vals := asList(infos)
	if response, err := client.Call("update_resinfo", vars, vals); err != nil {
		return err
	} else if response.Error != nil {
		return fmt.Errorf("rpc: %s %s", response.Error.Message, response.Error.Data)
	}

	return nil
}
