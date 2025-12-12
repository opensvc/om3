package object

import (
	"github.com/opensvc/om3/v3/util/key"
)

// Get returns a keyword value
func (t *core) Get(k key.T) (interface{}, error) {
	return t.config.Get(k), nil
}
