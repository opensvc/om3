package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceDetachModuleset is the options of the ComplianceDetachModuleset function.
	OptsNodeComplianceDetachModuleset struct {
		OptModuleset
	}
)

func (t Node) ComplianceDetachModuleset(options OptsNodeComplianceDetachModuleset) ([]string, error) {
	client, err := t.CollectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	err = comp.DetachModuleset(options.Moduleset)
	return nil, err
}
