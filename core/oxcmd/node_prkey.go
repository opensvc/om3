package oxcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
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
		nodeaction.WithRemoteNodes(t.NodeSelector),
	).Do()
}
