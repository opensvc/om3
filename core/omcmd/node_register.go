package omcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	CmdNodeRegister struct {
		OptsGlobal
		User         string
		Password     string
		App          string
		NodeSelector string
	}
)

func (t *CmdNodeRegister) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
			return nil, n.Register(ctx, t.User, t.Password, t.App)
		}),
	).Do()
}
