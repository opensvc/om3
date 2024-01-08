package commands

import (
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodePRKey struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodePRKey) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithLocalFunc(func() (any, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return n.PRKey()
		}),
	).Do()
}
