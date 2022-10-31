package commands

import (
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	CmdNodeEvents struct {
		OptsGlobal
	}
)

func (t *CmdNodeEvents) Run() error {
	var (
		err error
		c   *client.T
	)
	c, err = client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	streamer := c.NewGetEvents().SetRelatives(false)
	events, err := streamer.Do()
	if err != nil {
		return err
	}
	for m := range events {
		t.doOne(m)
	}
	return nil
}

func (t *CmdNodeEvents) doOne(e event.Event) {
	human := func() string {
		return event.Render(e)
	}
	if t.Format == output.JSON.String() {
		t.Format = output.JSONLine.String()
	}
	output.Renderer{
		Format:        t.Format,
		Color:         t.Color,
		Data:          e,
		HumanRenderer: human,
		Colorize:      rawconfig.Colorize,
	}.Print()
}
