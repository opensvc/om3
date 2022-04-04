package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceDetachModuleset is the options of the ComplianceDetachModuleset function.
	OptsObjectComplianceDetachModuleset struct {
		Global    OptsGlobal
		Moduleset OptModuleset
	}
)

func (t *Base) ComplianceDetachModuleset(options OptsObjectComplianceDetachModuleset) error {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.Path)
	return comp.DetachModuleset(options.Moduleset.Moduleset)
}
