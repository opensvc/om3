package oxcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeComplianceDetachRuleset struct {
		OptsGlobal
		Ruleset      string
		NodeSelector string
	}
)

func (t *CmdNodeComplianceDetachRuleset) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
	).Do()
}
