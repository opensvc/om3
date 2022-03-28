package object

import "opensvc.com/opensvc/util/compliance"

type (
	// OptsNodeComplianceShowRuleset is the options of the ComplianceShowRuleset function.
	OptsNodeComplianceShowRuleset struct {
		Global  OptsGlobal
		Ruleset OptRuleset
	}

	ComplianceShowRulesetResData struct {
	}
)

func (t Node) ComplianceShowRuleset(options OptsNodeComplianceShowRuleset) (compliance.Rulesets, error) {
	client, err := t.collectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	data, err := comp.GetRulesets()
	return data, err
}
