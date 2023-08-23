package commands

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodeGet struct {
		OptsGlobal
		OptsLock
		Eval    bool
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
			if t.Eval {
				return n.Eval(ctx, t.Keyword)
			} else {
				return n.Get(ctx, t.Keyword)
			}

		}),
	).Do()
}
