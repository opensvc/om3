package commands

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/path"
)

type (
	CmdObjectRun struct {
		OptsGlobal
		OptsLock
		OptsResourceSelector
		DryRun  bool
		Cron    bool
		Confirm bool
	}
)

func (t *CmdObjectRun) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRID(t.RID),
		objectaction.WithTag(t.Tag),
		objectaction.WithSubset(t.Subset),
		objectaction.WithLocal(t.Local),
		objectaction.WithFormat(t.Format),
		objectaction.WithColor(t.Color),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("run"),
		objectaction.WithLocalRun(func(ctx context.Context, p path.T) (interface{}, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			ctx = actioncontext.WithRID(ctx, t.RID)
			ctx = actioncontext.WithTag(ctx, t.Tag)
			ctx = actioncontext.WithSubset(ctx, t.Subset)
			ctx = actioncontext.WithCron(ctx, t.Cron)
			ctx = actioncontext.WithConfirm(ctx, t.Confirm)
			ctx = actioncontext.WithDryRun(ctx, t.DryRun)
			return nil, o.Run(ctx)
		}),
	).Do()
}
