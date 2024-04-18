package omcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/schedule"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdObjectPrintSchedule struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdObjectPrintSchedule) extract(selector string, c *client.T) (api.ScheduleList, error) {
	if t.Local {
		return t.extractLocal(selector)
	}
	if data, err := t.extractFromDaemons(selector, c); err == nil {
		return data, nil
	}
	return t.extractLocal(selector)
}

func (t *CmdObjectPrintSchedule) extractLocal(selector string) (api.ScheduleList, error) {
	data := api.ScheduleList{
		Kind: "ScheduleList",
	}
	sel := objectselector.New(
		selector,
		objectselector.WithLocal(true),
	)
	type scheduler interface {
		PrintSchedule() schedule.Table
	}
	paths, err := sel.Expand()
	if err != nil {
		return data, err
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
		for _, e := range i.PrintSchedule() {
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
	}
	return data, nil
}

func (t *CmdObjectPrintSchedule) extractFromDaemons(selector string, c *client.T) (api.ScheduleList, error) {
	var (
		errs error
		data api.ScheduleList
	)
	data.Kind = "ScheduleList"
	if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return data, err
	}
	paths, err := objectselector.New(selector, objectselector.WithClient(c)).Expand()
	if err != nil {
		return data, err
	}
	for _, nodename := range nodenames {
		for _, path := range paths {
			if d, err := t.extractFromDaemon(nodename, path, c); err != nil {
				errs = errors.Join(err)
			} else {
				data.Items = append(data.Items, d.Items...)
			}
		}
	}
	return data, errs
}

func (t *CmdObjectPrintSchedule) extractFromDaemon(nodename string, path naming.Path, c *client.T) (api.ScheduleList, error) {
	resp, err := c.GetObjectScheduleWithResponse(context.Background(), nodename, path.Namespace, path.Kind, path.Name)
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

func (t *CmdObjectPrintSchedule) Run(selector, kind string) error {
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
		DefaultOutput: "tab=OBJECT:meta.object,NODE:meta.node,ACTION:data.action,KEY:data.key,LAST_RUN_AT:data.last_run_at,NEXT_RUN_AT:data.next_run_at,SCHEDULE:data.schedule",
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return nil
}
