package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceFix is the options of the ComplianceFix function.
	OptsObjectComplianceFix struct {
		OptModuleset
		OptModule
		OptForce
		OptAttach
	}
)

func (t *Base) ComplianceFix(options OptsObjectComplianceFix) (*compliance.Run, error) {
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
	err = run.Fix()
	return run, err
}
