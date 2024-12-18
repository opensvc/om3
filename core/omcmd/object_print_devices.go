package omcmd

import (
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectdevice"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
)

type (
	CmdObjectPrintDevices struct {
		OptsGlobal
		NodeSelector string
		Roles        string
	}

	devicer interface {
		PrintDevices(roles objectdevice.Role) objectdevice.L
	}
)

func (t *CmdObjectPrintDevices) extract(selector string, c *client.T) (objectdevice.L, error) {
	if t.Local || (t.NodeSelector == "" && !clientcontext.IsSet()) {
		return t.extractLocal(selector)
	}
	if data, err := t.extractFromDaemon(selector, c); err == nil {
		return data, nil
	}
	if clientcontext.IsSet() {
		return objectdevice.NewList(), fmt.Errorf("can not fetch daemon data")
	}
	return t.extractLocal(selector)
}

func (t *CmdObjectPrintDevices) extractLocal(selector string) (objectdevice.L, error) {
	data := objectdevice.NewList()
	sel := objectselector.New(
		selector,
		objectselector.WithLocal(true),
	)
	paths, err := sel.Expand()
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
		table := i.PrintDevices(roles)
		data = data.Add(table)
	}
	return data, nil
}

func (t *CmdObjectPrintDevices) extractFromDaemon(selector string, c *client.T) (objectdevice.L, error) {
	data := objectdevice.NewList()
	/*
		req := c.NewGetDevicess()
		req.ObjectSelector = selector
		b, err := req.Do()
		if err != nil {
			return data, err
		}
		err = json.Unmarshal(b, &data)
		if err != nil {
			log.Debug().Err(err).Msg("unmarshal GET /schedules")
			return data, err
		}
	*/
	return data, nil
}

func (t *CmdObjectPrintDevices) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	c, err := client.New()
	if err != nil {
		return err
	}
	data, err := t.extract(mergedSelector, c)
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
