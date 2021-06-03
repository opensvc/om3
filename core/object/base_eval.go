package object

import (
	"opensvc.com/opensvc/util/key"
)

// OptsEval is the options of the Eval function of all base objects.
type OptsEval struct {
	Global      OptsGlobal
	Lock        OptsLocking
	Keyword     string `flag:"kw"`
	Impersonate string `flag:"impersonate"`
}

// Eval returns a keyword value
func (t *Base) Eval(options OptsEval) (interface{}, error) {
	k := key.Parse(options.Keyword)
	v, err := t.config.EvalAs(k, options.Impersonate)
	return v, err
}
