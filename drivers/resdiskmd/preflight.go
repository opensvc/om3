package resdiskmd

import (
	"context"
	"fmt"
)

// ProvisionConflicts says the array exists already, which the provision
// would adopt.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	if exists, err := t.exists(ctx); err == nil && exists {
		return []string{fmt.Sprintf("md %s exists", t.Label(ctx))}
	}
	return nil
}
