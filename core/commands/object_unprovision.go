package commands

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	CmdObjectUnprovision struct {
		OptsGlobal
		OptsAsync
		OptsLock
		OptsResourceSelector
		OptTo
		Force  bool
		Leader bool
		DryRun bool
	}
)

func (t *CmdObjectUnprovision) Run(selector, kind string) error {
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
		objectaction.WithRemoteAction("unprovision"),
		objectaction.WithAsyncTarget("unprovisioned"),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithDigest(),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			ctx = actioncontext.WithRID(ctx, t.RID)
			ctx = actioncontext.WithTag(ctx, t.Tag)
			ctx = actioncontext.WithSubset(ctx, t.Subset)
			ctx = actioncontext.WithTo(ctx, t.To)
			ctx = actioncontext.WithForce(ctx, t.Force)
			ctx = actioncontext.WithLeader(ctx, t.Leader)
			ctx = actioncontext.WithDryRun(ctx, t.DryRun)
			return nil, o.Unprovision(ctx)
		}),
	).Do()
}
