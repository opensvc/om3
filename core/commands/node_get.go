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
		Eval        bool
		Impersonate string
		Keywords    []string
	}
)

func (t *CmdNodeGet) Run() error {
	return nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			for _, s := range t.Keywords {
				if t.Eval {
					if t.Impersonate != "" {
						return n.EvalAs(ctx, s, t.Impersonate)
					} else {
						return n.Eval(ctx, s)
					}
				} else {
					return n.Get(ctx, s)
				}
			}
			return nil, nil
		}),
	).Do()
}
