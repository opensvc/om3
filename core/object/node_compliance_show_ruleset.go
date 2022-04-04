package object

import "opensvc.com/opensvc/util/compliance"

type (
	// OptsNodeComplianceShowRuleset is the options of the ComplianceShowRuleset function.
	OptsNodeComplianceShowRuleset struct {
		Global  OptsGlobal
		Ruleset OptRuleset
	}
)

func (t Node) ComplianceShowRuleset(options OptsNodeComplianceShowRuleset) (compliance.Rulesets, error) {
	client, err := t.CollectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	data, err := comp.GetRulesets()
	return data, err
}
