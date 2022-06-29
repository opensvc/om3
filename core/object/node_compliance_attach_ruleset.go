package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceAttachRuleset is the options of the ComplianceAttachRuleset function.
	OptsNodeComplianceAttachRuleset struct {
		OptRuleset
	}
)

func (t Node) ComplianceAttachRuleset(options OptsNodeComplianceAttachRuleset) ([]string, error) {
	client, err := t.CollectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	err = comp.AttachRuleset(options.Ruleset)
	return nil, err
}
