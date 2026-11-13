package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectComplianceAttachRuleset struct {
		OptsGlobal
		Ruleset string
	}
)

// Run attaches the ruleset to the selected objects, on the collector.
func (t *CmdObjectComplianceAttachRuleset) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return commoncmd.ObjectsAPIDo(mergedSelector, t.OptsGlobal.OptsGlobal, commoncmd.ObjectComplianceRulesetAttachment(t.Ruleset, true))
}
