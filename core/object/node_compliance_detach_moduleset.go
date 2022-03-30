package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceDetachModuleset is the options of the ComplianceDetachModuleset function.
	OptsNodeComplianceDetachModuleset struct {
		Global    OptsGlobal
		Moduleset OptModuleset
	}
)

func (t Node) ComplianceDetachModuleset(options OptsNodeComplianceDetachModuleset) ([]string, error) {
	client, err := t.collectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	err = comp.DetachModuleset(options.Moduleset.Moduleset)
	return nil, err
}
