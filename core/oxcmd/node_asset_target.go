package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNodeAssetTarget struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeAssetTarget) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}

	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}

	sel := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c))
	nodenames, err := sel.Expand()
	if err != nil {
		return err
	}

	l := make(api.SANPathItems, 0)
	for _, nodename := range nodenames {
		response, err := c.GetNodeDiscoverTargetWithResponse(context.Background(), nodename)
		if err != nil {
			return err
		}
		switch {
		case response.JSON200 != nil:
			l = append(l, response.JSON200.Items...)
		case response.JSON400 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON400)
		case response.JSON401 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON401)
		case response.JSON403 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON403)
		case response.JSON500 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON500)
		default:
			return fmt.Errorf("%s: unexpected response: %s", nodename, response.Status())
		}
	}
	defaultOutput := "tab=INITIATOR:data.initiator,TARGET:data.target"
	output.Renderer{
		DefaultOutput: defaultOutput,
		Output:        t.Output,
		Color:         t.Color,
		Data:          api.SANPathList{Items: l, Kind: "SANPathList"},
		Colorize:      rawconfig.Colorize,
	}.Print()

	return nil
}
