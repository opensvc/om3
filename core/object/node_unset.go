package object

import (
	"opensvc.com/opensvc/util/key"
)

// Unset gets a keyword value
func (t *Node) Unset(options OptsUnset) error {
	return unset(t.config, options)
}

func (t *Node) UnsetKeys(kws ...key.T) error {
	return unsetKeys(t.config, kws...)
}
