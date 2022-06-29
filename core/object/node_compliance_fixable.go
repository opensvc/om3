package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceFixable is the options of the ComplianceFixable function.
	OptsNodeComplianceFixable struct {
		OptModuleset
		OptModule
		OptForce
		OptAttach
	}
)

func (t Node) ComplianceFixable(options OptsNodeComplianceFixable) (*compliance.Run, error) {
	client, err := t.CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	run := comp.NewRun()
	run.SetModulesetsExpr(options.Moduleset)
	run.SetModulesExpr(options.Module)
	run.SetForce(options.Force)
	run.SetAttach(options.Attach)
	err = run.Fixable()
	return run, err
}
