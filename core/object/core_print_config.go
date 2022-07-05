package object

import (
	"opensvc.com/opensvc/core/rawconfig"
)

// PrintConfig gets a keyword value
func (t *core) PrintConfig() (rawconfig.T, error) {
	return t.config.Raw(), nil
}

func (t *core) EvalConfig() (rawconfig.T, error) {
	return t.config.RawEvaluated()
}

func (t *core) EvalConfigAs(nodename string) (rawconfig.T, error) {
	return t.config.RawEvaluatedAs(nodename)
}
