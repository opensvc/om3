package resdiskloop

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/util/file"
)

// ProvisionConflicts says the file the loop device is backed by exists
// already, which the provision would adopt.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	if t.File != "" && file.Exists(t.File) {
		return []string{fmt.Sprintf("file %s exists", t.File)}
	}
	return nil
}
