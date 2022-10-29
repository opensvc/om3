package commands

import (
	"context"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	CmdObjectFreeze struct {
		OptsGlobal
		OptsAsync
	}
)

func (t *CmdObjectFreeze) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithLocal(t.Local),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithFormat(t.Format),
		objectaction.WithColor(t.Color),
		objectaction.WithServer(t.Server),
		objectaction.WithAsyncTarget("frozen"),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("freeze"),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.NewActor(p)
			if err != nil {
				return nil, err
			}
			return nil, o.Freeze(context.Background())
		}),
	).Do()
}
