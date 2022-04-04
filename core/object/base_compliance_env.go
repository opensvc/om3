package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceEnv is the options of the ComplianceEnv function.
	OptsObjectComplianceEnv struct {
		Global    OptsGlobal
		Moduleset OptModuleset
		Module    OptModule
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
	run.SetModulesetsExpr(options.Moduleset.Moduleset)
	run.SetModulesExpr(options.Module.Module)
	return run.Env()
}
