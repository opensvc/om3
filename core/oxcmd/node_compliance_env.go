package oxcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
)

type (
	CmdNodeComplianceEnv struct {
		OptsGlobal
		Moduleset    string
		Module       string
		NodeSelector string
	}
)

func (t *CmdNodeComplianceEnv) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
	).Do()
}
