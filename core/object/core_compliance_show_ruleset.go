package object

import (
	"opensvc.com/opensvc/util/compliance"
)

func (t *core) ComplianceShowRuleset() (compliance.Rulesets, error) {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.path)
	data, err := comp.GetRulesets()
	return data, err
}
