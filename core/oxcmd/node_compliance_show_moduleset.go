package oxcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
)

type (
	CmdNodeComplianceShowModuleset struct {
		OptsGlobal
		Moduleset    string
		NodeSelector string
	}
)

func (t *CmdNodeComplianceShowModuleset) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
	).Do()
}
