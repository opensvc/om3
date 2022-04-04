package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceListModuleset is the options of the ComplianceListModuleset function.
	OptsObjectComplianceListModuleset struct {
		Global    OptsGlobal
		Moduleset OptModuleset
	}
)

func (t *Base) ComplianceListModuleset(options OptsObjectComplianceListModuleset) ([]string, error) {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.Path)
	data, err := comp.ListModulesets(options.Moduleset.Moduleset)
	if err != nil {
		return nil, err
	}
	return data, nil
}
