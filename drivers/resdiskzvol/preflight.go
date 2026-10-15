package resdiskzvol

import (
	"context"
	"fmt"
)

// ProvisionConflicts says the volume exists already, which the provision
// would adopt.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	if exists, err := t.hasIt(); err == nil && exists {
		return []string{fmt.Sprintf("zvol %s exists", t.Name)}
	}
	return nil
}
