package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceAttachRuleset is the options of the ComplianceAttachRuleset function.
	OptsObjectComplianceAttachRuleset struct {
		OptRuleset
	}
)

func (t *Base) ComplianceAttachRuleset(options OptsObjectComplianceAttachRuleset) error {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.Path)
	return comp.AttachRuleset(options.Ruleset)
}
