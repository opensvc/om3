package rescontainerkvm

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/resource"
)

// ProvisionConflicts says a domain of the name is defined already, when the
// provision would create one, as virtinst says: a domain defined by a
// sysadmin, which a resource with no virtinst is configured on, is its own.
func (t *T) ProvisionConflicts(ctx context.Context) []string {
	if !resource.ProvisionCreates(ctx) {
		return nil
	}
	if t.hasConfigFile() {
		return []string{fmt.Sprintf("domain %s is defined", t.Name)}
	}
	return nil
}
