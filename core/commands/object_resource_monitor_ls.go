package commands

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectResourceMonitorLs struct {
		OptsGlobal
		OptsResourceSelector
	}
)

func (t *CmdObjectResourceMonitorLs) Run(selector, kind string) error {
	var (
		data any
		err  error
	)
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")

	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	params := api.GetResourceMonitorParams{Path: &mergedSelector}
	if t.NodeSelector != "" {
		params.Node = &t.NodeSelector
	}
	if t.RID != "" {
		params.Resource = &t.RID
	}
	// TODO: add subset and tag params
	resp, err := c.GetResourceMonitorWithResponse(context.Background(), &params)
	if err != nil {
		return fmt.Errorf("api: %w", err)
	}
	switch resp.StatusCode() {
	case 200:
		data = *resp.JSON200
	case 400:
		data = *resp.JSON400
	case 401:
		data = *resp.JSON401
	case 403:
		data = *resp.JSON403
	case 500:
		data = *resp.JSON500
	}
	renderer := output.Renderer{
		DefaultOutput: "tab=OBJECT:meta.object,NODE:meta.node,RID:meta.rid,RESTART_REMAINING:data.restart.remaining,RESTART_LAST_AT:data.restart.last_at",
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}
	renderer.Print()
	return nil
}
