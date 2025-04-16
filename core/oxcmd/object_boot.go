package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/xsession"
)

type (
	CmdObjectBoot struct {
		OptsGlobal
		commoncmd.OptsResourceSelector
		commoncmd.OptTo
		commoncmd.OptsLock
		NodeSelector string
	}
)

func (t *CmdObjectBoot) Run(selector, kind string) error {
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithRID(t.RID),
		objectaction.WithTag(t.Tag),
		objectaction.WithSubset(t.Subset),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteFunc(func(ctx context.Context, p naming.Path, nodename string) (interface{}, error) {
			c, err := client.New()
			if err != nil {
				return nil, err
			}
			params := api.PostInstanceActionBootParams{}
			if t.OptsResourceSelector.RID != "" {
				params.Rid = &t.OptsResourceSelector.RID
			}
			if t.OptsResourceSelector.Subset != "" {
				params.Subset = &t.OptsResourceSelector.Subset
			}
			if t.OptsResourceSelector.Tag != "" {
				params.Tag = &t.OptsResourceSelector.Tag
			}
			if t.OptTo.To != "" {
				params.To = &t.OptTo.To
			}
			{
				sid := xsession.ID
				params.RequesterSid = &sid
			}
			response, err := c.PostInstanceActionBootWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
			if err != nil {
				return nil, err
			}
			switch {
			case response.JSON200 != nil:
				return *response.JSON200, nil
			case response.JSON401 != nil:
				return nil, fmt.Errorf("%s: node %s: %s", p, nodename, *response.JSON401)
			case response.JSON403 != nil:
				return nil, fmt.Errorf("%s: node %s: %s", p, nodename, *response.JSON403)
			case response.JSON500 != nil:
				return nil, fmt.Errorf("%s: node %s: %s", p, nodename, *response.JSON500)
			default:
				return nil, fmt.Errorf("%s: node %s: unexpected response: %s", p, nodename, response.Status())
			}
		}),
	).Do()
}
