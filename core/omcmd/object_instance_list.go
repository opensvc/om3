package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectInstanceList struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdObjectInstanceList) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "*/"+kind+"/*")

	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.GetInstancesParams{Path: &mergedSelector}
	if t.NodeSelector != "" {
		params.Node = &t.NodeSelector
	}
	resp, err := c.GetInstancesWithResponse(context.Background(), &params)
	if err != nil {
		return fmt.Errorf("api: %w", err)
	}
	var pb *api.Problem
	switch resp.StatusCode() {
	case 200:
		output.Renderer{
			DefaultOutput: "tab=OBJECT:meta.object,NODE:meta.node,AVAIL:data.status.avail",
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
