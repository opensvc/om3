package object

import "opensvc.com/opensvc/util/compliance"

func (t Node) ComplianceShowRuleset() (compliance.Rulesets, error) {
	client, err := t.CollectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	data, err := comp.GetRulesets()
	return data, err
}
