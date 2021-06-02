package object

import (
	"opensvc.com/opensvc/core/rawconfig"
)

// OptsPrintConfig is the options of the PrintConfig object method.
type OptsPrintConfig struct {
	Global      OptsGlobal
	Lock        OptsLocking
	Eval        bool   `flag:"eval"`
	Impersonate string `flag:"impersonate"`
}

// PrintConfig gets a keyword value
func (t *Base) PrintConfig(options OptsPrintConfig) (rawconfig.T, error) {
	if options.Eval {
		// TODO
		return rawconfig.T{}, nil
	}
	return t.config.Raw(), nil
}
