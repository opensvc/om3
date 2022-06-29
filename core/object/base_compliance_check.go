package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceCheck is the options of the ComplianceCheck function.
	OptsObjectComplianceCheck struct {
		OptModuleset
		OptModule
		OptForce
		OptAttach
	}
)

func (t *Base) ComplianceCheck(options OptsObjectComplianceCheck) (*compliance.Run, error) {
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
	err = run.Check()
	return run, err
}
