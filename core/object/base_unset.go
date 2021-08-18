package object

import (
	"opensvc.com/opensvc/util/key"
)

// OptsUnset is the options of the Unset object method.
type OptsUnset struct {
	Global   OptsGlobal
	Lock     OptsLocking
	Keywords []string `flag:"kws"`
}

// Unset gets a keyword value
func (t *Base) Unset(options OptsUnset) error {
	kws := make([]key.T, 0)
	for _, kw := range options.Keywords {
		kws = append(kws, key.Parse(kw))
	}
	return t.UnsetKeys(kws...)
}

func (t *Base) UnsetKeys(kws ...key.T) error {
	changes := 0
	for _, k := range kws {
		changes += t.config.Unset(k)
	}
	if changes > 0 {
		return t.config.Commit()
	}
	return nil
}
