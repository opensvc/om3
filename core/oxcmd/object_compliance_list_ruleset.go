package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectComplianceListRuleset struct {
		OptsGlobal
		Ruleset string
	}
)

// Run lists the rulesets the collector makes available to the selected
// objects.
func (t *CmdObjectComplianceListRuleset) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return commoncmd.ObjectsAPIDo(mergedSelector, t.OptsGlobal.OptsGlobal, commoncmd.ObjectComplianceRulesets(t.Ruleset))
}
