package object

import (
	"opensvc.com/opensvc/util/compliance"
)

func (t *Base) ComplianceListModules() ([]string, error) {
	comp := compliance.New()
	comp.SetObjectPath(t.Path)
	data, err := comp.ListModuleNames()
	if err != nil {
		return nil, err
	}
	return data, nil

}
