package object

import (
	"opensvc.com/opensvc/util/compliance"
	"opensvc.com/opensvc/util/xstrings"
)

type (
	// OptsNodeComplianceEnv is the options of the ComplianceEnv function.
	OptsNodeComplianceEnv struct {
		Global    OptsGlobal
		Moduleset OptModuleset
		Module    OptModule
	}

	ComplianceEnvResData struct {
	}
)

func (t Node) ComplianceEnv(options OptsNodeComplianceEnv) (compliance.Envs, error) {
	modsets := xstrings.Split(options.Moduleset.Moduleset, ",")
	mods := xstrings.Split(options.Module.Module, ",")
	client, err := t.collectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	return comp.GetEnvs(modsets, mods)
}
