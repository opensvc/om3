package oxcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeComplianceListModules struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeComplianceListModules) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
	).Do()
}
