package oxcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
)

type (
	CmdNodePrintConfig struct {
		OptsGlobal
		Eval         bool
		Impersonate  string
		NodeSelector string
	}
)

func (t *CmdNodePrintConfig) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
	).Do()
}
