package commands

import (
	"context"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/objectlogger"
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
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithServer(t.Server),
		objectaction.WithAsyncTarget("thawed"),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("unfreeze"),
		objectaction.WithLocalRun(func(ctx context.Context, p naming.Path) (interface{}, error) {
			logger := objectlogger.New(p,
				objectlogger.WithColor(t.Color != "no"),
				objectlogger.WithConsoleLog(t.Log != ""),
				objectlogger.WithLogFile(true),
				objectlogger.WithSessionLogFile(true),
			)
			o, err := object.NewActor(p,
				object.WithLogger(logger),
			)
			if err != nil {
				return nil, err
			}
			return nil, o.Unfreeze(ctx)
		}),
	).Do()
}
