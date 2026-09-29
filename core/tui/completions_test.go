package tui

import (
	"slices"
	"testing"
)

// The provision of an instance and of resources offers --state-only, to mark
// provisioned what a sysadmin provisioned by hand.
func TestProvisionCompletesStateOnly(t *testing.T) {
	for name, tree := range map[string]node{"instance": nodeDoInstance, "resource": nodeDoResource} {
		candidates := tree["provision"].Candidates("do provision ", "", map[string]bool{})
		if !slices.Contains(candidates, "do provision --state-only") {
			t.Errorf("%s provision candidates: %v", name, candidates)
		}
	}
}
