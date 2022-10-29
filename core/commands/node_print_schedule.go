package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/schedule"
)

type (
	CmdNodePrintSchedule struct {
		OptsGlobal
	}
)

func (t *CmdNodePrintSchedule) extract(c *client.T) schedule.Table {
	if t.Local {
		return t.extractLocal()
	}
	if data, err := t.extractFromDaemon(c); err == nil {
		return data
	}
	if clientcontext.IsSet() {
		log.Error().Msg("can not fetch daemon data")
		return schedule.NewTable()
	}
	return t.extractLocal()
}

func (t *CmdNodePrintSchedule) extractLocal() schedule.Table {
	data := schedule.NewTable()
	obj, err := object.NewNode()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	table := obj.PrintSchedule()
	data = data.Add(table)
	return data
}

func (t *CmdNodePrintSchedule) extractFromDaemon(c *client.T) (schedule.Table, error) {
	data := schedule.NewTable()
	req := c.NewGetSchedules()
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

func (t *CmdNodePrintSchedule) Run() error {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	data := t.extract(c)

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
