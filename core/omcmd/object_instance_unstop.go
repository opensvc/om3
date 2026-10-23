package omcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/core/xerrors"
)

type (
	CmdObjectInstanceUnstop struct {
		OptsGlobal
		commoncmd.OptsAsync
		commoncmd.OptsLock
		commoncmd.OptsResourceSelector
		NodeSelector string
	}
)

func (t *CmdObjectInstanceUnstop) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRID(t.RID),
		objectaction.WithTag(t.Tag),
		objectaction.WithSubset(t.Subset),
		objectaction.WithOutput(t.Output),
		objectaction.WithSort(t.Sort),
		objectaction.WithColor(t.Color),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithAsyncFollow(t.Follow),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteFunc(func(ctx context.Context, p naming.Path, nodename string) (interface{}, error) {
			c, err := client.New()
			if err != nil {
				return nil, err
			}
			return commoncmd.PostInstanceActionUnstop(ctx, c, p, nodename, t.OptsResourceSelector)
		}),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			type unstopper interface {
				Unstop(context.Context) error
			}
			o, err := object.New(p)
			if err != nil {
				return nil, err
			}
			i, ok := o.(unstopper)
			if !ok {
				return nil, xerrors.InstanceActionNotSupported
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			return nil, i.Unstop(ctx)
		}),
	).Do()
}
