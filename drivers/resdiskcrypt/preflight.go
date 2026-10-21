package resdiskcrypt

import (
	"context"
	"fmt"
)

// ProvisionConflicts says the encrypted device exists already, which the
// provision would adopt.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	if exists, err := t.exists(ctx); err == nil && exists {
		return []string{fmt.Sprintf("%s exists", t.getName())}
	}
	return nil
}
