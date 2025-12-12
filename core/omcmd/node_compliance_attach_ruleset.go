package omcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
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
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			comp, err := n.NewCompliance()
			if err != nil {
				return nil, err
			}
			return nil, comp.AttachRuleset(t.Ruleset)
		}),
	).Do()
}
