package object

import (
	"opensvc.com/opensvc/config"
)

// PrintConfigOptions is the options of the PrintConfig object method.
type OptsPrintConfig struct {
	Global      OptsGlobal
	Lock        OptsLocking
	Eval        bool   `flag:"eval"`
	Impersonate string `flag:"impersonate"`
}

// PrintConfig gets a keyword value
func (t *Base) PrintConfig(options OptsPrintConfig) (config.Raw, error) {
	if options.Eval {
		// TODO
		return config.Raw{}, nil
	} else {
		return t.config.Raw(), nil
	}
}
