package object

import (
	"opensvc.com/opensvc/util/key"
)

// Get returns a keyword value
func (t *Node) Get(options OptsGet) (interface{}, error) {
	k := key.Parse(options.Keyword)
	if options.Eval {
		v, err := t.mergedConfig.EvalAs(k, options.Impersonate)
		return v, err
	}
	return t.config.Get(k), nil
}

// Eval returns a keyword value
func (t *Node) Eval(options OptsEval) (interface{}, error) {
	k := key.Parse(options.Keyword)
	return t.mergedConfig.EvalAs(k, options.Impersonate)
}
