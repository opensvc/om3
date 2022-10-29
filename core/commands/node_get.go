package commands

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	CmdNodeGet struct {
		OptsGlobal
		OptsLock
		Keyword string
	}
)

func (t *CmdNodeGet) Run() error {
	return nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("get"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"kw": t.Keyword,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			return n.Get(ctx, t.Keyword)

		}),
	).Do()
}
