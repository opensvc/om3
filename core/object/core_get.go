package object

import (
	"opensvc.com/opensvc/util/key"
)

// Get returns a keyword value
func (t *core) Get(k key.T) (interface{}, error) {
	return t.config.Get(k), nil
}
