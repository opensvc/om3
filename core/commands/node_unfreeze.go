package commands

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/xsession"
)

type CmdNodeUnfreeze struct {
	OptsGlobal
	OptsAsync
}

func (t *CmdNodeUnfreeze) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithAsyncWait(t.Wait),
		nodeaction.WithAsyncTime(t.Time),
		nodeaction.WithAsyncWatch(t.Watch),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteRun(func(ctx context.Context, nodename string) (interface{}, error) {
			c, err := client.New(client.WithURL(nodename))
			if err != nil {
				return nil, err
			}
			params := api.PostNodeActionUnfreezeParams{}
			{
				sid := xsession.ID
				params.RequesterSid = &sid
			}
			response, err := c.PostNodeActionUnfreezeWithResponse(ctx, &params)
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
		nodeaction.WithLocal(true),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return nil, n.Unfreeze()
		}),
	).Do()
}
