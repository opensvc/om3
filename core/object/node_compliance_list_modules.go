package object

import (
	"opensvc.com/opensvc/util/compliance"
)

func (t Node) ComplianceListModules() ([]string, error) {
	comp := compliance.New()
	data, err := comp.ListModuleNames()
	if err != nil {
		return nil, err
	}
	return data, nil
}
