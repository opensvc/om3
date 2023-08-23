package commands

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodeEval struct {
		OptsGlobal
		OptsLock
		Keyword     string
		Impersonate string
	}
)

func (t *CmdNodeEval) Run() error {
	return nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("push_asset"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"kw":          t.Keyword,
			"impersonate": t.Impersonate,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			return n.EvalAs(ctx, t.Keyword, t.Impersonate)
		}),
	).Do()
}
