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

func (t *core) ComplianceDetachRuleset(options OptsObjectComplianceDetachRuleset) error {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.path)
	return comp.DetachRuleset(options.Ruleset)
}
