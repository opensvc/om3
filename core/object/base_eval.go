package object

import (
	"opensvc.com/opensvc/core/xconfig"
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
	switch err.(type) {
	case xconfig.ErrPostponedRef:
		// example: disk#1.exposed_devs[0]
		t.configureResources()
		v, err = t.config.EvalAs(k, options.Impersonate)
	}
	return v, err
}
