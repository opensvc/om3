package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceAuto is the options of the ComplianceAuto function.
	OptsObjectComplianceAuto struct {
		OptModuleset
		OptModule
		OptForce
		OptAttach
	}
)

func (t *core) ComplianceAuto(options OptsObjectComplianceAuto) (*compliance.Run, error) {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.path)
	run := comp.NewRun()
	run.SetModulesetsExpr(options.Moduleset)
	run.SetModulesExpr(options.Module)
	run.SetForce(options.Force)
	run.SetAttach(options.Attach)
	err = run.Auto()
	return run, err
}
