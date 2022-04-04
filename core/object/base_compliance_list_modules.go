package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceListModules is the options of the ComplianceListModules function.
	OptsObjectComplianceListModules struct {
		Global OptsGlobal
	}
)

func (t *Base) ComplianceListModules(options OptsObjectComplianceListModules) ([]string, error) {
	comp := compliance.New()
	comp.SetObjectPath(t.Path)
	data, err := comp.ListModuleNames()
	if err != nil {
		return nil, err
	}
	return data, nil

}
