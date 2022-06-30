package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceDetachModuleset is the options of the ComplianceDetachModuleset function.
	OptsObjectComplianceDetachModuleset struct {
		OptModuleset
	}
)

func (t *core) ComplianceDetachModuleset(options OptsObjectComplianceDetachModuleset) error {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.path)
	return comp.DetachModuleset(options.Moduleset)
}
