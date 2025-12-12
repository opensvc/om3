package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	CmdNodeCapabilitiesScan struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeCapabilitiesScan) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return n.ScanCapabilities()
		}),
		nodeaction.WithRemoteFunc(func(ctx context.Context, nodename string) (interface{}, error) {
			c, err := client.New()
			if err != nil {
				return nil, err
			}
			params := api.PostNodeActionScanCapabilitiesParams{}
			{
				sid := xsession.ID
				params.RequesterSid = &sid
			}
			response, err := c.PostNodeActionScanCapabilitiesWithResponse(ctx, nodename, &params)
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
}
