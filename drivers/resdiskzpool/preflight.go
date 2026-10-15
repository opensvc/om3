package resdiskzpool

import (
	"context"
	"fmt"
)

// ProvisionConflicts says the pool exists already, which the provision would
// adopt.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	if exists, err := t.hasIt(ctx); err == nil && exists {
		return []string{fmt.Sprintf("zpool %s exists", t.Name)}
	}
	return nil
}
