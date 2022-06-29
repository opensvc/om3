package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceEnv is the options of the ComplianceEnv function.
	OptsObjectComplianceEnv struct {
		OptModuleset
		OptModule
	}
)

func (t *Base) ComplianceEnv(options OptsObjectComplianceEnv) (compliance.Envs, error) {
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
	return run.Env()
}
