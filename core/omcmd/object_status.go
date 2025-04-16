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
	CmdObjectStatus struct {
		OptsGlobal
		commoncmd.OptsLock
		Refresh      bool
		Monitor      bool
		NodeSelector string
	}
)

func (t *CmdObjectStatus) Run(selector, kind string) error {
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.Local),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			o, err := object.NewCore(p)
			if err != nil {
				return nil, err
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			if t.Monitor {
				return o.MonitorStatus(ctx)
			} else if t.Refresh {
				return o.FreshStatus(ctx)
			} else {
				return o.Status(ctx)
			}
		}),
	).Do()
}
