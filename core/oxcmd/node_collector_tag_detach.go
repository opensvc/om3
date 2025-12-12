package oxcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeCollectorTagDetach struct {
		OptsGlobal
		Name         string
		NodeSelector string
	}
)

func (t *CmdNodeCollectorTagDetach) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithRemoteNodes(t.NodeSelector),
	).Do()
}
