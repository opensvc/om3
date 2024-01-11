package oxcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
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

func (t *CmdNodePrintSchedule) extract(c *client.T, nodename string) (api.ScheduleList, error) {
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
	var errs error
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	var data api.ScheduleList
	for i, nodename := range nodenames {
		if d, err := t.extract(c, nodename); err != nil {
			errs = errors.Join(err)
		} else if i == 0 {
			data = d
		} else {
			data.Items = append(data.Items, d.Items...)
		}

	}
	output.Renderer{
		DefaultOutput: "tab=NODE:meta.node,ACTION:data.action,LAST_RUN_AT:data.last_run_at,NEXT_RUN_AT:data.next_run_at,SCHEDULE:data.schedule",
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return errs
}
