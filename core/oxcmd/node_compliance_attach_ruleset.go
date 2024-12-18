package oxcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
)

type (
	CmdNodeComplianceAttachRuleset struct {
		OptsGlobal
		NodeSelector string
		Ruleset      string
	}
)

func (t *CmdNodeComplianceAttachRuleset) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
	).Do()
}
