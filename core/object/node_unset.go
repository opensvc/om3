package object

import (
	"context"

	"opensvc.com/opensvc/util/key"
)

// Unset gets a keyword value
func (t *Node) Unset(ctx context.Context, kws ...key.T) error {
	return unsetKeys(t.config, kws...)
}
