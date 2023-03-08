package commands

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/path"
)

type (
	CmdObjectSetProvisioned struct {
		OptsGlobal
		OptsLock
		OptsResourceSelector
		DryRun bool
	}
)

func (t *CmdObjectSetProvisioned) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("set provisioned"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"rid": t.RID,
		}),
		objectaction.WithLocalRun(func(ctx context.Context, p path.T) (interface{}, error) {
			o, err := object.NewActor(p,
				object.WithConsoleLog(t.Log != ""),
				object.WithConsoleColor(t.Color != "no"),
			)

			if err != nil {
				return nil, err
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			ctx = actioncontext.WithRID(ctx, t.RID)
			ctx = actioncontext.WithTag(ctx, t.Tag)
			ctx = actioncontext.WithSubset(ctx, t.Subset)
			ctx = actioncontext.WithDryRun(ctx, t.DryRun)
			return nil, o.SetProvisioned(ctx)
		}),
	).Do()
}
