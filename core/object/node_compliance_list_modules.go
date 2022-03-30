package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceListModules is the options of the ComplianceListModules function.
	OptsNodeComplianceListModules struct {
		Global OptsGlobal
	}
)

func (t Node) ComplianceListModules(options OptsNodeComplianceListModules) ([]string, error) {
	comp := compliance.New()
	data, err := comp.ListModules()
	if err != nil {
		return nil, err
	}
	return data, nil
}
