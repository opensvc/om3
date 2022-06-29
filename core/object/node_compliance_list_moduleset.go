package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceListModuleset is the options of the ComplianceListModuleset function.
	OptsNodeComplianceListModuleset struct {
		OptModuleset
	}
)

func (t Node) ComplianceListModuleset(options OptsNodeComplianceListModuleset) ([]string, error) {
	client, err := t.CollectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	data, err := comp.ListModulesets(options.Moduleset)
	if err != nil {
		return nil, err
	}
	return data, nil
}
