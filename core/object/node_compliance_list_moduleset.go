package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceListModuleset is the options of the ComplianceListModuleset function.
	OptsNodeComplianceListModuleset struct {
		Global    OptsGlobal
		Moduleset OptModuleset
	}
)

func (t Node) ComplianceListModuleset(options OptsNodeComplianceListModuleset) ([]string, error) {
	client, err := t.collectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	data, err := comp.ListModulesets(options.Moduleset.Moduleset)
	if err != nil {
		return nil, err
	}
	return data, nil
}
