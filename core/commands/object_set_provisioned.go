package commands

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/objectlogger"
)

type (
	CmdObjectSetProvisioned struct {
		OptsGlobal
		OptsLock
		OptsResourceSelector
	}
)

func (t *CmdObjectSetProvisioned) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("set provisioned"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"rid": t.RID,
		}),
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
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			return nil, o.SetProvisioned(ctx)
		}),
	).Do()
}
