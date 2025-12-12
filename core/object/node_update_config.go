package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/util/key"
)

// Update updates applies configurations changes in the configuration file.
func (t *Node) Update(ctx context.Context, deleteSections []string, unsetKeys []key.T, keyOps []keyop.T) (err error) {
	return t.config.Update(deleteSections, unsetKeys, keyOps)
}
