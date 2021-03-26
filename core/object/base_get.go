package object

// OptsBaseGet is the options of the Get function of all base objects.
type OptsGet struct {
	Global      OptsGlobal
	Lock        OptsLocking
	Keyword     string `flag:"kw"`
	Eval        bool   `flag:"eval"`
	Impersonate bool   `flag:"impersonate"`
}

// Get returns a keyword value
func (t *Base) Get(options OptsGet) (interface{}, error) {
	if options.Eval {
		return t.config.Eval(options.Keyword)
	} else {
		return t.config.Get(options.Keyword)
	}
}
