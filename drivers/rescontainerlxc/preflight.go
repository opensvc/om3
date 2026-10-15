package rescontainerlxc

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/resource"
)

// ProvisionConflicts says a container of the name exists already, when the
// provision would create one, as a template or a rootfs to create it from
// says: a container made by a sysadmin, which a resource setting none of them
// is configured on, is its own.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	if !resource.ProvisionCreates(ctx) {
		return nil
	}
	if t.exists(ctx) {
		return []string{fmt.Sprintf("container %s exists", t.Name)}
	}
	return nil
}
