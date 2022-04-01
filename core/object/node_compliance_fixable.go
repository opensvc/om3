package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceFixable is the options of the ComplianceFixable function.
	OptsNodeComplianceFixable struct {
		Global    OptsGlobal
		Moduleset OptModuleset
		Module    OptModule
		Force     OptForce
		Attach    OptAttach
	}
)

func (t Node) ComplianceFixable(options OptsNodeComplianceFixable) (*compliance.Run, error) {
	client, err := t.collectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	run := comp.NewRun()
	run.SetModulesetsExpr(options.Moduleset.Moduleset)
	run.SetModulesExpr(options.Module.Module)
	run.SetForce(options.Force.Force)
	run.SetAttach(options.Attach.Attach)
	err = run.Fixable()
	return run, err
}
