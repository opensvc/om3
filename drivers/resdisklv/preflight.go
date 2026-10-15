package resdisklv

import (
	"context"
	"fmt"
)

// ProvisionConflicts says the logical volume exists already, which the
// provision would adopt.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	if exists, err := t.lv().Exists(ctx); err == nil && exists {
		return []string{fmt.Sprintf("logical volume %s/%s exists", t.VGName, t.LVName)}
	}
	return nil
}
