package oxcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
)

type (
	CmdNodePushPatch struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodePushPatch) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
	).Do()
}
