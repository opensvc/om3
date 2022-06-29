package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceFixable is the options of the ComplianceFixable function.
	OptsObjectComplianceFixable struct {
		OptModuleset
		OptModule
		OptForce
		OptAttach
	}
)

func (t *Base) ComplianceFixable(options OptsObjectComplianceFixable) (*compliance.Run, error) {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.Path)
	run := comp.NewRun()
	run.SetModulesetsExpr(options.Moduleset)
	run.SetModulesExpr(options.Module)
	run.SetForce(options.Force)
	run.SetAttach(options.Attach)
	err = run.Fixable()
	return run, err
}
