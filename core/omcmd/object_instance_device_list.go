package omcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectdevice"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	CmdObjectInstanceDeviceList struct {
		OptsGlobal
		NodeSelector string
		Roles        string
	}

	devicer interface {
		PrintDevices(context.Context, objectdevice.Role) objectdevice.L
	}
)

func (t *CmdObjectInstanceDeviceList) extractLocal(ctx context.Context, selector string) (objectdevice.L, error) {
	data := objectdevice.NewList()
	sel := objectselector.New(
		selector,
		objectselector.WithLocal(true),
	)
	paths, err := sel.MustExpand()
	if err != nil {
		return data, err
	}
	for _, p := range paths {
		obj, err := object.New(p)
		if err != nil {
			continue
		}
		i, ok := obj.(devicer)
		if !ok {
			continue
		}
		roles := objectdevice.ParseRoles(t.Roles)
		table := i.PrintDevices(ctx, roles)
		data = data.Add(table)
	}
	return data, nil
}

// Run lists the devices of the local instances of the selected objects, or
// with a node selector the ones of their instances of the nodes selected,
// each asked of its node.
func (t *CmdObjectInstanceDeviceList) Run(kind string) error {
	ctx := context.Background()
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	if t.NodeSelector != "" {
		return commoncmd.InstanceDeviceList(mergedSelector, t.NodeSelector, t.Roles, t.OptsGlobal.OptsGlobal)
	}
	data, err := t.extractLocal(ctx, mergedSelector)
	if err != nil {
		return err
	}
	return output.Renderer{
		DefaultOutput: "tab=OBJECT:path,RESOURCE:rid,DRIVER_GROUP:driver.group,DRIVER_NAME:driver.name,ROLE:role,DEVICE:device",
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
}
