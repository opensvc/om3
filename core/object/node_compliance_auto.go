package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceAuto is the options of the ComplianceAuto function.
	OptsNodeComplianceAuto struct {
		OptModuleset
		OptModule
		OptForce
		OptAttach
	}
)

func (t Node) ComplianceAuto(options OptsNodeComplianceAuto) (*compliance.Run, error) {
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
	err = run.Auto()
	return run, err
}
