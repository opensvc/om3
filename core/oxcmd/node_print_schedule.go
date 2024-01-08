package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/schedule"
)

type (
	CmdNodePrintSchedule struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodePrintSchedule) extract(c *client.T) (schedule.Table, error) {
	data := schedule.NewTable()
	return data, fmt.Errorf("todo")
}

func (t *CmdNodePrintSchedule) Run() error {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	data, err := t.extract(c)

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
