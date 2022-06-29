package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceEnv is the options of the ComplianceEnv function.
	OptsNodeComplianceEnv struct {
		OptModuleset
		OptModule
	}
)

func (t Node) ComplianceEnv(options OptsNodeComplianceEnv) (compliance.Envs, error) {
	client, err := t.CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	run := comp.NewRun()
	run.SetModulesetsExpr(options.Moduleset)
	run.SetModulesExpr(options.Module)
	return run.Env()
}
