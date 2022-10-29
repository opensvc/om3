package commands

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	CmdObjectDelete struct {
		OptsGlobal
		OptsLock
		DryRun bool
		RID    string
	}
)

func (t *CmdObjectDelete) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("delete"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"rid": t.RID,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewConfigurer(p)
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
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
