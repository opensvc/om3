package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceFix is the options of the ComplianceFix function.
	OptsNodeComplianceFix struct {
		OptModuleset
		OptModule
		OptForce
		OptAttach
	}
)

func (t Node) ComplianceFix(options OptsNodeComplianceFix) (*compliance.Run, error) {
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
	err = run.Fix()
	return run, err
}
