package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceListRuleset is the options of the ComplianceListRuleset function.
	OptsObjectComplianceListRuleset struct {
		OptRuleset
	}
)

func (t *core) ComplianceListRuleset(options OptsObjectComplianceListRuleset) ([]string, error) {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.path)
	data, err := comp.ListRulesets(options.Ruleset)
	if err != nil {
		return nil, err
	}
	return data, nil
}
