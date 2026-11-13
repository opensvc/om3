package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectComplianceDetachRuleset struct {
		OptsGlobal
		Ruleset string
	}
)

// Run detaches the ruleset from the selected objects, on the collector.
func (t *CmdObjectComplianceDetachRuleset) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return commoncmd.ObjectsAPIDo(mergedSelector, t.OptsGlobal.OptsGlobal, commoncmd.ObjectComplianceRulesetAttachment(t.Ruleset, false))
}
