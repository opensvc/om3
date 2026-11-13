package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeaction"
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
		nodeaction.WithDefaultOutput(""),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (any, error) {
			comp, err := newNodeCompliance()
			if err != nil {
				return nil, err
			}
			return nil, comp.AttachRuleset(t.Ruleset)
		}),
		nodeaction.WithRemoteFunc(commoncmd.NodeComplianceRulesetAttachmentRemote(t.Ruleset, true)),
	).Do()
}
