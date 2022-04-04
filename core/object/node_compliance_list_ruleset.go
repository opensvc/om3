package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceListRuleset is the options of the ComplianceListRuleset function.
	OptsNodeComplianceListRuleset struct {
		Global  OptsGlobal
		Ruleset OptRuleset
	}
)

func (t Node) ComplianceListRuleset(options OptsNodeComplianceListRuleset) ([]string, error) {
	client, err := t.CollectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	data, err := comp.ListRulesets(options.Ruleset.Ruleset)
	if err != nil {
		return nil, err
	}
	return data, nil
}
