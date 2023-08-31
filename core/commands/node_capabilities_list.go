package commands

import (
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodeCapabilitiesList struct {
		OptsGlobal
	}
)

func (t *CmdNodeCapabilitiesList) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),

		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteAction("node print capabilities"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Output,
		}),

		nodeaction.WithLocal(t.Local),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return n.PrintCapabilities()
		}),
	).Do()
}
