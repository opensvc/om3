package oxcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeComplianceListRuleset struct {
		OptsGlobal
		Ruleset      string
		NodeSelector string
	}
)

func (t *CmdNodeComplianceListRuleset) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
	).Do()
}
