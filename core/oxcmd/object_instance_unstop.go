package oxcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectInstanceUnstop struct {
		OptsGlobal
		commoncmd.OptsAsync
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
	).Do()
}
