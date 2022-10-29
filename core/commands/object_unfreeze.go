package commands

import (
	"context"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	CmdObjectUnfreeze struct {
		OptsGlobal
		OptsAsync
	}
)

func (t *CmdObjectUnfreeze) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithLocal(t.Local),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithFormat(t.Format),
		objectaction.WithColor(t.Color),
		objectaction.WithServer(t.Server),
		objectaction.WithAsyncTarget("thawed"),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("unfreeze"),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			return nil, o.Unfreeze(context.Background())
		}),
	).Do()
}
