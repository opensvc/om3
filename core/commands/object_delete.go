package commands

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/path"
)

type (
	CmdObjectDelete struct {
		OptsGlobal
		OptsLock
		RID string
	}
)

func (t *CmdObjectDelete) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("delete"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"rid": t.RID,
		}),
		objectaction.WithLocalRun(func(ctx context.Context, p path.T) (interface{}, error) {
			o, err := object.NewConfigurer(p)
			if err != nil {
				return nil, err
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			if t.RID != "" {
				return nil, o.DeleteSection(ctx, t.RID)
			} else {
				return nil, o.Delete(ctx)
			}
		}),
	).Do()
}
