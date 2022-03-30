package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceDetachRuleset is the options of the ComplianceDetachRuleset function.
	OptsNodeComplianceDetachRuleset struct {
		Global  OptsGlobal
		Ruleset OptRuleset
	}
)

func (t Node) ComplianceDetachRuleset(options OptsNodeComplianceDetachRuleset) ([]string, error) {
	client, err := t.collectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	err = comp.DetachRuleset(options.Ruleset.Ruleset)
	return nil, err
}
