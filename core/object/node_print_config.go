package object

import (
	"opensvc.com/opensvc/core/rawconfig"
)

// PrintConfig gets a keyword value
func (t *Node) PrintConfig(options OptsPrintConfig) (rawconfig.T, error) {
	if options.Eval {
		return t.config.RawEvaluatedAs(options.Impersonate)
	}
	return t.config.Raw(), nil
}
