package rescontainerocibase

import (
	"context"
	"fmt"
)

// ProvisionConflicts says a container of the name exists already, running or
// not: its start would take it for a leftover of its own and remove it, or
// run the one found.
func (t *BT) ProvisionConflicts(ctx context.Context) []string {
	if t.executer == nil {
		return nil
	}
	inspect, err := t.ContainerInspectRefresh(ctx)
	if err != nil || inspect == nil {
		return nil
	}
	return []string{fmt.Sprintf("container %s exists", t.ContainerName())}
}
