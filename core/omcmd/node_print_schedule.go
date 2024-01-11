package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNodePrintSchedule struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodePrintSchedule) extract(c *client.T) (api.ScheduleList, error) {
	if t.Local {
		return t.extractLocal()
	}
	if data, err := t.extractFromDaemons(c); err == nil {
		return data, nil
	}
	return t.extractLocal()
}

func (t *CmdNodePrintSchedule) extractLocal() (api.ScheduleList, error) {
	var data api.ScheduleList
	data.Kind = "ScheduleList"

	n, err := object.NewNode()
	if err != nil {
		return data, err
	}

	for _, e := range n.Schedules() {
		item := api.ScheduleItem{
			Kind: "ScheduleItem",
			Meta: api.InstanceMeta{
				Node:   e.Node,
				Object: e.Path.String(),
			},
			Data: api.Schedule{
				Action:             e.Action,
				Key:                e.Key,
				LastRunAt:          e.LastRunAt,
				LastRunFile:        e.LastRunFile,
				LastSuccessFile:    e.LastSuccessFile,
				NextRunAt:          e.NextRunAt,
				RequireCollector:   e.RequireCollector,
				RequireProvisioned: e.RequireProvisioned,
				Schedule:           e.Schedule,
			},
		}
		data.Items = append(data.Items, item)
	}

	return data, nil
}

func (t *CmdNodePrintSchedule) extractFromDaemons(c *client.T) (api.ScheduleList, error) {
	var (
		errs error
		data api.ScheduleList
	)
	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return data, err
	}
	for i, nodename := range nodenames {
		if d, err := t.extractFromDaemon(c, nodename); err != nil {
			errs = errors.Join(err)
		} else if i == 0 {
			data = d
		} else {
			data.Items = append(data.Items, d.Items...)
		}

	}
	return data, errs
}

func (t *CmdNodePrintSchedule) extractFromDaemon(c *client.T, nodename string) (api.ScheduleList, error) {
	resp, err := c.GetNodeScheduleWithResponse(context.Background(), nodename)
	if err != nil {
		return api.ScheduleList{}, err
	}
	switch resp.StatusCode() {
	case 200:
		return *resp.JSON200, nil
	case 401:
		return api.ScheduleList{}, fmt.Errorf("%s: %s", nodename, *resp.JSON401)
	case 403:
		return api.ScheduleList{}, fmt.Errorf("%s: %s", nodename, *resp.JSON403)
	default:
		return api.ScheduleList{}, fmt.Errorf("%s: unexpected statuscode: %s", nodename, resp.Status())
	}
}

func (t *CmdNodePrintSchedule) Run() error {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	data, err := t.extract(c)
	output.Renderer{
		DefaultOutput: "tab=NODE:meta.node,ACTION:data.action,LAST_RUN_AT:data.last_run_at,NEXT_RUN_AT:data.next_run_at,SCHEDULE:data.schedule",
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Items:         data.Items,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return err
}
