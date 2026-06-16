package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	CmdNodePushDisks struct {
		OptsGlobal
		NodeSelector                string
		DryRun                      bool
		IgnoreNoCollectorConfigured bool
	}
)

func (t *CmdNodePushDisks) Run() error {
	if t.DryRun {
		if t.NodeSelector == "" {
			fmt.Println(hostname.Hostname())
			return nil
		}
		c, err := client.New()
		if err != nil {
			return err
		}
		nodes, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
		if err != nil {
			return err
		}
		for _, n := range nodes {
			fmt.Println(n)
		}
		return nil
	}

	err := nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithRemoteFunc(func(ctx context.Context, nodename string) (interface{}, error) {
			c, err := client.New()
			if err != nil {
				return nil, err
			}
			params := api.PostNodeActionPushDiskParams{}
			{
				sid := xsession.Sid().UUID()
				params.SessionId = &sid
			}
			response, err := c.PostNodeActionPushDiskWithResponse(ctx, nodename, &params)
			if err != nil {
				return nil, err
			}
			switch {
			case response.JSON200 != nil:
				return *response.JSON200, nil
			case response.JSON401 != nil:
				return nil, fmt.Errorf("node %s: %s", nodename, *response.JSON401)
			case response.JSON403 != nil:
				return nil, fmt.Errorf("node %s: %s", nodename, *response.JSON403)
			case response.JSON500 != nil:
				return nil, fmt.Errorf("node %s: %s", nodename, *response.JSON500)
			default:
				return nil, fmt.Errorf("node %s: unexpected response: %s", nodename, response.Status())
			}
		}),
	).Do()

	if err != nil && t.IgnoreNoCollectorConfigured && isNoCollectorError(err) {
		return nil
	}
	return err
}
