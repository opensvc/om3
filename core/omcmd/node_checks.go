package omcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	CmdNodeChecks struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeChecks) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
			return n.Checks(ctx)
		}),
	).Do()
}
