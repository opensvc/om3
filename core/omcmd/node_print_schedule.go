package omcmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdNodePrintSchedule struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodePrintSchedule) extract(c *client.T) (api.ScheduleList, error) {
	var data api.ScheduleList
	data.Kind = "ScheduleList"

	var (
		items api.ScheduleItems
		err   error
	)

	if t.Local {
		items, err = t.extractLocal()
	} else {
		items, err = t.extractFromDaemon(c)
	}

	data.Items = items

	return data, err
}

func (t *CmdNodePrintSchedule) extractLocal() (api.ScheduleItems, error) {
	var items api.ScheduleItems

	n, err := object.NewNode()
	if err != nil {
		return items, err
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
		items = append(items, item)
	}
	return items, nil
}

func (t *CmdNodePrintSchedule) extractFromDaemon(c *client.T) (api.ScheduleItems, error) {
	var l api.ScheduleItems

	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return l, err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*1)
	defer cancel()

	q := make(chan api.ScheduleItems)
	errC := make(chan error)
	doneC := make(chan string)
	todo := len(nodenames)
	var needDoLocal bool

	for _, nodename := range nodenames {
		go func(nodename string) {
			defer func() { doneC <- nodename }()
			resp, err := c.GetNodeScheduleWithResponse(context.Background(), nodename)
			if err != nil {
				if nodename == hostname.Hostname() {
					needDoLocal = true
				}
				errC <- err
				return
			}
			switch resp.StatusCode() {
			case 200:
				q <- resp.JSON200.Items
			case 401:
				errC <- fmt.Errorf("%s: %s", nodename, *resp.JSON401)
			case 403:
				errC <- fmt.Errorf("%s: %s", nodename, *resp.JSON403)
			default:
				errC <- fmt.Errorf("%s: unexpected statuscode: %s", nodename, resp.Status())
			}
		}(nodename)
	}

	var (
		errs error
		done int
	)

	for {
		select {
		case err := <-errC:
			errs = errors.Join(errs, err)
		case items := <-q:
			l = append(l, items...)
		case <-doneC:
			done++
			if done == todo {
				goto out
			}
		case <-ctx.Done():
			errs = errors.Join(errs, ctx.Err())
			goto out
		}
	}

out:

	if needDoLocal {
		items, err := t.extractLocal()
		if err != nil {
			errs = errors.Join(errs, err)
		} else {
			l = append(l, items...)
		}
	}
	return l, errs
}

func (t *CmdNodePrintSchedule) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}

	data, err := t.extract(c)

	output.Renderer{
		DefaultOutput: "tab=NODE:meta.node,ACTION:data.action,LAST_RUN_AT:data.last_run_at,NEXT_RUN_AT:data.next_run_at,SCHEDULE:data.schedule",
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return err
}
