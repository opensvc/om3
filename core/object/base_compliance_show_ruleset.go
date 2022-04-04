package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceShowRuleset is the options of the ComplianceEnv function.
	OptsObjectComplianceShowRuleset struct {
		Global  OptsGlobal
		Ruleset OptRuleset
	}
)

func (t *Base) ComplianceShowRuleset(options OptsObjectComplianceShowRuleset) (compliance.Rulesets, error) {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.Path)
	data, err := comp.GetRulesets()
	return data, err
}
