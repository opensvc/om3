package oxcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
)

type (
	CmdNodePushDisks struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodePushDisks) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
	).Do()
}
