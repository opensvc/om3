package commands

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/util/key"
)

type (
	CmdNodeUnset struct {
		OptsGlobal
		OptsLock
		Keywords []string
	}
)

func (t *CmdNodeUnset) Run() error {
	return nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("unset"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"kw": t.Keywords,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			kws := key.ParseL(t.Keywords)
			return nil, n.Unset(ctx, kws...)
		}),
	).Do()
}
