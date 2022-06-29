package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceCheck is the options of the ComplianceCheck function.
	OptsNodeComplianceCheck struct {
		OptModuleset
		OptModule
		OptForce
		OptAttach
	}
)

func (t Node) ComplianceCheck(options OptsNodeComplianceCheck) (*compliance.Run, error) {
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
	err = run.Check()
	return run, err
}
