package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceFix is the options of the ComplianceFix function.
	OptsNodeComplianceFix struct {
		Global    OptsGlobal
		Moduleset OptModuleset
		Module    OptModule
		Force     OptForce
		Attach    OptAttach
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
	run.SetModulesetsExpr(options.Moduleset.Moduleset)
	run.SetModulesExpr(options.Module.Module)
	run.SetForce(options.Force.Force)
	run.SetAttach(options.Attach.Attach)
	err = run.Fix()
	return run, err
}
