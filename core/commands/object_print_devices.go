package commands

import (
	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectdevice"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	CmdObjectPrintDevices struct {
		OptsGlobal
		Roles string
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
		return objectdevice.NewList(), errors.Errorf("can not fetch daemon data")
	}
	return t.extractLocal(selector)
}

func (t *CmdObjectPrintDevices) extractLocal(selector string) (objectdevice.L, error) {
	data := objectdevice.NewList()
	sel := objectselector.NewSelection(
		selector,
		objectselector.SelectionWithLocal(true),
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
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	data, err := t.extract(mergedSelector, c)
	if err != nil {
		return err
	}

	output.Renderer{
		Format:   t.Format,
		Color:    t.Color,
		Data:     data,
		Colorize: rawconfig.Colorize,
		HumanRenderer: func() string {
			return data.Render()
		},
	}.Print()
	return nil
}
