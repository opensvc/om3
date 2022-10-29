package commands

import (
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	CmdNodeChecks struct {
		OptsGlobal
	}
)

func (t *CmdNodeChecks) Run() error {
	return nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("checks"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return n.Checks()
		}),
	).Do()
}
