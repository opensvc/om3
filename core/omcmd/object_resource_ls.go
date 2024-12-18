package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectResourceLs struct {
		OptsGlobal
		OptsResourceSelector
		NodeSelector string
	}
)

func (t *CmdObjectResourceLs) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")

	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.GetResourcesParams{Path: &mergedSelector}
	if t.NodeSelector != "" {
		params.Node = &t.NodeSelector
	}
	if t.RID != "" {
		params.Resource = &t.RID
	}
	// TODO: add subset and tag params
	resp, err := c.GetResourcesWithResponse(context.Background(), &params)
	if err != nil {
		return fmt.Errorf("api: %w", err)
	}
	var pb *api.Problem
	switch resp.StatusCode() {
	case 200:
		output.Renderer{
			DefaultOutput: "tab=OBJECT:meta.object,NODE:meta.node,RID:meta.rid,TYPE:data.status.type,STATUS:data.status.status,IS_MONITORED:data.config.is_monitored,IS_DISABLED:data.config.is_disabled,IS_STANDBY:data.config.is_standby,RESTART:data.config.restart,RESTART_REMAINING:data.monitor.restart.remaining",
			Output:        t.Output,
			Color:         t.Color,
			Data:          resp.JSON200,
			Colorize:      rawconfig.Colorize,
		}.Print()
		return nil
	case 400:
		pb = resp.JSON400
	case 401:
		pb = resp.JSON401
	case 403:
		pb = resp.JSON403
	case 500:
		pb = resp.JSON500
	}
	return fmt.Errorf("%s", pb)
}
