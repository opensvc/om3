package oxcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
)

type (
	CmdNodeDrivers struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeDrivers) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteNodes(t.NodeSelector),
	).Do()
}
