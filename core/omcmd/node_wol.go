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
	CmdNodeWol struct {
		OptsGlobal
		Local        bool
		NodeSelector string
		Macs         []string
		Broadcasts   []string
		Ports        []int
	}
)

func (t *CmdNodeWol) Run() error {
	return nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithRemoteFunc(func(ctx context.Context, nodename string) (interface{}, error) {
			return nodeWolRemote(ctx, nodename, t.Macs, t.Broadcasts, t.Ports)
		}),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return nil, n.Wol(t.Macs, t.Broadcasts, t.Ports)
		}),
	).Do()
}

func nodeWolRemote(ctx context.Context, nodename string, macs, broadcasts []string, ports []int) (interface{}, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	params := api.PostNodeActionWolParams{
		Mac:       macs,
		Broadcast: broadcasts,
	}
	if len(ports) > 0 {
		params.Port = &ports
	}
	{
		sessionID := xsession.SessionID().UUID()
		params.SessionID = &sessionID
	}
	response, err := c.PostNodeActionWolWithResponse(ctx, nodename, &params)
	if err != nil {
		return nil, err
	}
	switch {
	case response.JSON200 != nil:
		return *response.JSON200, nil
	case response.JSON400 != nil:
		return nil, fmt.Errorf("node %s: %s", nodename, *response.JSON400)
	case response.JSON401 != nil:
		return nil, fmt.Errorf("node %s: %s", nodename, *response.JSON401)
	case response.JSON403 != nil:
		return nil, fmt.Errorf("node %s: %s", nodename, *response.JSON403)
	case response.JSON500 != nil:
		return nil, fmt.Errorf("node %s: %s", nodename, *response.JSON500)
	default:
		return nil, fmt.Errorf("node %s: unexpected response: %s", nodename, response.Status())
	}
}
