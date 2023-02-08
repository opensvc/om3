package object

import (
	"github.com/opensvc/om3/core/rawconfig"
)

// PrintConfig gets a keyword value
func (t *Node) PrintConfig() (rawconfig.T, error) {
	return t.config.Raw(), nil
}

func (t *Node) EvalConfig() (rawconfig.T, error) {
	return t.config.RawEvaluatedAs("")
}

func (t *Node) EvalConfigAs(impersonate string) (rawconfig.T, error) {
	return t.config.RawEvaluatedAs(impersonate)
}
