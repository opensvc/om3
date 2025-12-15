package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
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

func (t *CmdObjectInstanceDeviceList) extract(ctx context.Context, selector string, c *client.T) (objectdevice.L, error) {
	if t.NodeSelector == "" {
		return t.extractLocal(ctx, selector)
	}
	return t.extractFromDaemon(ctx, selector, c)
}

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

func (t *CmdObjectInstanceDeviceList) extractFromDaemon(ctx context.Context, selector string, c *client.T) (objectdevice.L, error) {
	data := objectdevice.NewList()
	return data, fmt.Errorf("TODO")
}

func (t *CmdObjectInstanceDeviceList) Run(kind string) error {
	ctx := context.Background()
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	c, err := client.New()
	if err != nil {
		return err
	}
	data, err := t.extract(ctx, mergedSelector, c)
	if err != nil {
		return err
	}
	output.Renderer{
		DefaultOutput: "tab=OBJECT:path,RESOURCE:rid,DRIVER_GROUP:driver.group,DRIVER_NAME:driver.name,ROLE:role,DEVICE:device",
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()

	return nil
}
