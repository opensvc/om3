package commands

import (
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodePRKey struct {
		OptsGlobal
	}
)

func (t *CmdNodePRKey) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteAction("node prkey"),
		nodeaction.WithRemoteOptions(map[string]any{
			"format": t.Output,
		}),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithLocalRun(func() (any, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return n.PRKey()
		}),
	).Do()
}
