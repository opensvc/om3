package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNodeList struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeList) Run() error {
	var (
		err      error
		selector string
	)
	c, err := client.New()
	if err != nil {
		return err
	}
	if t.NodeSelector == "" {
		selector = "*"
	} else {
		selector = t.NodeSelector
	}
	resp, err := c.GetNodesWithResponse(context.Background(), &api.GetNodesParams{Node: &selector})
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
	case 401:
		return fmt.Errorf("%s", *resp.JSON401)
	case 403:
		return fmt.Errorf("%s", *resp.JSON403)
	default:
		return fmt.Errorf("unexpected statuscode: %s", resp.Status())
	}

	output.Renderer{
		DefaultOutput: "tab=NAME:meta.node,AGENT:data.status.agent,STATE:data.monitor.state",
		Output:        t.Output,
		Color:         t.Color,
		Data:          *resp.JSON200,
	}.Print()
	return nil
}
