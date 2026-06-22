package omcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/core/xerrors"
)

type (
	CmdObjectInstancePGUpdate struct {
		OptsGlobal
		commoncmd.OptsAsync
		commoncmd.OptsEncap
		commoncmd.OptsLock
		commoncmd.OptsResourceSelector
	}
)

func (t *CmdObjectInstancePGUpdate) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRID(t.RID),
		objectaction.WithTag(t.Tag),
		objectaction.WithSubset(t.Subset),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithSlaves(t.Slaves),
		objectaction.WithAllSlaves(t.AllSlaves),
		objectaction.WithMaster(t.Master),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			type pgUpdater interface {
				PGUpdate(context.Context) error
			}
			o, err := object.New(p)
			if err != nil {
				return nil, err
			}
			i, ok := o.(pgUpdater)
			if !ok {
				return nil, xerrors.InstanceActionNotSupported
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			return nil, i.PGUpdate(ctx)
		}),
	).Do()
}
