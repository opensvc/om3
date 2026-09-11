package omcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	CmdNodePushArrays struct {
		OptsGlobal
		Local                       bool
		NodeSelector                string
		Array                       string
		IgnoreNoCollectorConfigured bool
	}
)

func (t *CmdNodePushArrays) Run() error {
	err := nodeaction.New(
		nodeaction.WithLocal(true),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return n.PushArrays(context.Background(), t.Array)
		}),
	).Do()

	if err != nil && t.IgnoreNoCollectorConfigured && isNoCollectorError(err) {
		return nil
	}
	return err
}
