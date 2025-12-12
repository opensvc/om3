package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/keyop"
)

// Set sets a keyword value
func (t *Node) Set(ctx context.Context, kops ...keyop.T) error {
	return t.config.Set(kops...)
}
