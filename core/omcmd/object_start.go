package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/xsession"
)

type (
	CmdObjectStart struct {
		OptsGlobal
		OptsAsync
		OptsLock
		OptsResourceSelector
		OptTo
		Force           bool
		DisableRollback bool
		NodeSelector    string
	}
)

func (t *CmdObjectStart) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRID(t.RID),
		objectaction.WithTag(t.Tag),
		objectaction.WithSubset(t.Subset),
		objectaction.WithLocal(t.Local),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget("started"),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteFunc(func(ctx context.Context, p naming.Path, nodename string) (interface{}, error) {
			c, err := client.New(client.WithURL(t.Server))
			if err != nil {
				return nil, err
			}
			params := api.PostInstanceActionStartParams{}
			if t.Force {
				v := true
				params.Force = &v
			}
			if t.DisableRollback {
				v := true
				params.DisableRollback = &v
			}
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
			response, err := c.PostInstanceActionStartWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
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
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			ctx = actioncontext.WithTo(ctx, t.To)
			ctx = actioncontext.WithForce(ctx, t.Force)
			ctx = actioncontext.WithRollbackDisabled(ctx, t.DisableRollback)
			return nil, o.Start(ctx)
		}),
	).Do()
}
