package omcmd

import (
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdObjectScheduleList struct {
		OptsGlobal
		Local        bool
		NodeSelector string
	}
)

func (t *CmdObjectScheduleList) extract(selector string, c *client.T) (api.ScheduleList, error) {
	if t.Local {
		return t.extractLocal(selector)
	}
	if data, err := t.extractFromDaemons(selector, c); err == nil {
		return data, nil
	}
	return t.extractLocal(selector)
}

func (t *CmdObjectScheduleList) extractLocal(selector string) (api.ScheduleList, error) {
	data := api.ScheduleList{
		Kind: "ScheduleList",
	}
	sel := objectselector.New(
		selector,
		objectselector.WithLocal(true),
	)
	type scheduler interface {
		Schedules() schedule.Table
	}
	paths, err := sel.MustExpand()
	if err != nil {
		return data, err
	}
	var errs error
	for _, p := range paths {
		obj, err := object.New(p)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", p, err))
			continue
		}
		i, ok := obj.(scheduler)
		if !ok {
			continue
		}
		for _, e := range i.Schedules() {
			item := api.ScheduleItem{
				Kind: "ScheduleItem",
				Meta: api.InstanceMeta{
					Node:   e.Node,
					Object: e.Path.String(),
				},
				Data: api.Schedule{
					Action:             e.Action,
					Key:                e.Key,
					LastRunAt:          nullableScheduleTime(e.LastRunAt),
					MaxParallel:        e.MaxParallel,
					NextRunAt:          nullableScheduleTime(e.NextRunAt),
					Require:            e.Require,
					RequireCollector:   e.RequireCollector,
					RequireProvisioned: e.RequireProvisioned,
					Schedule:           e.Schedule,
					On:                 append([]string{}, e.On...),
					Pace:               e.Pace,
				},
			}
			data.Items = append(data.Items, item)
		}
	}
	return data, errs
}

func (t *CmdObjectScheduleList) extractFromDaemons(selector string, c *client.T) (api.ScheduleList, error) {
	if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	return commoncmd.InstanceSchedules(c, selector, t.NodeSelector)
}

func (t *CmdObjectScheduleList) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	c, err := client.New()
	if err != nil {
		return err
	}
	data, err := t.extract(mergedSelector, c)
	err = errors.Join(err, output.Renderer{
		DefaultOutput: commoncmd.ScheduleListOutput,
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print())
	return err
}
