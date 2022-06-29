package object

import (
	"opensvc.com/opensvc/util/compliance"
)

func (t *core) ComplianceListModules() ([]string, error) {
	comp := compliance.New()
	comp.SetObjectPath(t.path)
	data, err := comp.ListModuleNames()
	if err != nil {
		return nil, err
	}
	return data, nil

}
