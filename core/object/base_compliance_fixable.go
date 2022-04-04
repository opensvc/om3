package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceFixable is the options of the ComplianceFixable function.
	OptsObjectComplianceFixable struct {
		Global    OptsGlobal
		Moduleset OptModuleset
		Module    OptModule
		Force     OptForce
		Attach    OptAttach
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
	run.SetModulesetsExpr(options.Moduleset.Moduleset)
	run.SetModulesExpr(options.Module.Module)
	err = run.Fixable()
	return run, err
}
