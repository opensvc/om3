package omcmd

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectSyncIngest struct {
		OptsGlobal
		commoncmd.OptsLock
		commoncmd.OptsResourceSelector
	}
)

func (t *CmdObjectSyncIngest) Run(selector, kind string) error {
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRID(t.RID),
		objectaction.WithTag(t.Tag),
		objectaction.WithSubset(t.Subset),
		objectaction.WithLocal(true),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (any, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			return nil, o.SyncIngest(ctx)
		}),
	).Do()
}
