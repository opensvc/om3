package resdiskvg

import (
	"context"
	"fmt"
)

// ProvisionConflicts says the volume group of the name exists already, which
// the provision would adopt.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	if exists, err := t.vg().Exists(ctx); err == nil && exists {
		return []string{fmt.Sprintf("volume group %s exists", t.VGName)}
	}
	return nil
}
