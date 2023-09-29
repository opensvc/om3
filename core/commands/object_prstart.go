package commands

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/naming"
)

type (
	CmdObjectPRStart struct {
		OptsGlobal
		OptsLock
		OptsResourceSelector
		OptTo
		Force bool
	}
)

func (t *CmdObjectPRStart) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRID(t.RID),
		objectaction.WithTag(t.Tag),
		objectaction.WithSubset(t.Subset),
		objectaction.WithLocal(true),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("prstart"),
		objectaction.WithProgress(!t.Quiet && t.Log == ""),
		objectaction.WithLocalRun(func(ctx context.Context, p naming.Path) (any, error) {
			o, err := object.NewActor(p,
				object.WithConsoleLog(t.Log != ""),
				object.WithConsoleColor(t.Color != "no"),
			)

			if err != nil {
				return nil, err
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			ctx = actioncontext.WithTo(ctx, t.To)
			ctx = actioncontext.WithForce(ctx, t.Force)
			return nil, o.PRStart(ctx)
		}),
	).Do()
}
