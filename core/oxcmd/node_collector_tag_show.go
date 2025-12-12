package oxcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeCollectorTagShow struct {
		OptsGlobal
		Verbose      bool
		NodeSelector string
	}
)

func (t *CmdNodeCollectorTagShow) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
	).Do()
}
