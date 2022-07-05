package object

import (
	"context"

	"opensvc.com/opensvc/core/keyop"
)

// Set sets a keyword value
func (t *Node) Set(ctx context.Context, kops ...keyop.T) error {
	return setKeys(t.config, kops...)
}
