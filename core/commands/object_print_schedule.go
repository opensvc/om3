package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/schedule"
)

type (
	CmdObjectPrintSchedule struct {
		OptsGlobal
	}
)

func (t *CmdObjectPrintSchedule) extract(selector string, c *client.T) schedule.Table {
	if t.Local {
		return t.extractLocal(selector)
	}
	if data, err := t.extractFromDaemon(selector, c); err == nil {
		return data
	}
	if clientcontext.IsSet() {
		log.Error().Msg("can not fetch daemon data")
		return schedule.NewTable()
	}
	return t.extractLocal(selector)
}

func (t *CmdObjectPrintSchedule) extractLocal(selector string) schedule.Table {
	data := schedule.NewTable()
	sel := objectselector.NewSelection(
		selector,
		objectselector.SelectionWithLocal(true),
	)
	type scheduler interface {
		PrintSchedule() schedule.Table
	}
	paths, err := sel.Expand()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return data
	}
	for _, p := range paths {
		obj, err := object.New(p)
		if err != nil {
			continue
		}
		i, ok := obj.(scheduler)
		if !ok {
			continue
		}
		table := i.PrintSchedule()
		data = data.Add(table)
	}
	return data
}

func (t *CmdObjectPrintSchedule) extractFromDaemon(selector string, c *client.T) (schedule.Table, error) {
	data := schedule.NewTable()
	req := c.NewGetSchedules()
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
	return data, nil
}

func (t *CmdObjectPrintSchedule) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	data := t.extract(mergedSelector, c)

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
