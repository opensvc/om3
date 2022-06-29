package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceDetachRuleset is the options of the ComplianceDetachRuleset function.
	OptsObjectComplianceDetachRuleset struct {
		OptRuleset
	}
)

func (t *Base) ComplianceDetachRuleset(options OptsObjectComplianceDetachRuleset) error {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.Path)
	return comp.DetachRuleset(options.Ruleset)
}
