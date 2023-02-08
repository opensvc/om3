package object

import (
	"context"

	"github.com/opensvc/om3/util/key"
)

// Unset gets a keyword value
func (t *Node) Unset(ctx context.Context, kws ...key.T) error {
	return unsetKeys(t.config, kws...)
}
