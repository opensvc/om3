package object

import "opensvc.com/opensvc/util/key"

// OptsGet is the options of the Get function of all base objects.
type OptsGet struct {
	Global      OptsGlobal
	Lock        OptsLocking
	Keyword     string `flag:"kw"`
	Eval        bool   `flag:"eval"`
	Impersonate bool   `flag:"impersonate"`
}

// Get returns a keyword value
func (t *Base) Get(options OptsGet) (interface{}, error) {
	k := key.Parse(options.Keyword)
	if options.Eval {
		return t.config.Eval(k)
	}
	return t.config.Get(k), nil
}
