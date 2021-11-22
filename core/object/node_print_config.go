package object

import (
	"opensvc.com/opensvc/core/rawconfig"
)

// PrintConfig gets a keyword value
func (t *Node) PrintConfig(options OptsPrintConfig) (rawconfig.T, error) {
	if options.Eval {
		// TODO
		return rawconfig.T{}, nil
	}
	return t.config.Raw(), nil
}
