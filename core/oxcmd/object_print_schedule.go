package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/schedule"
)

type (
	CmdObjectPrintSchedule struct {
		OptsGlobal
	}
)

func (t *CmdObjectPrintSchedule) extract(selector string, c *client.T) (schedule.Table, error) {
	if data, err := t.extractFromDaemon(selector, c); err == nil {
		return data, nil
	} else {
		return schedule.NewTable(), err
	}
}

func (t *CmdObjectPrintSchedule) extractFromDaemon(selector string, c *client.T) (schedule.Table, error) {
	data := schedule.NewTable()
	return data, fmt.Errorf("todo")
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
		Output:   t.Output,
		Color:    t.Color,
		Data:     data,
		Colorize: rawconfig.Colorize,
		HumanRenderer: func() string {
			return data.Render()
		},
	}.Print()
	return nil
}
