package tui

import (
	"slices"
	"testing"
)

// The provision and the unprovision of an instance and of resources offer
// --state-only, to mark provisioned or unprovisioned what a sysadmin
// provisioned or unprovisioned by hand.
func TestProvisionCompletesStateOnly(t *testing.T) {
	for name, tree := range map[string]node{"instance": nodeDoInstance, "resource": nodeDoResource} {
		for _, action := range []string{"provision", "unprovision"} {
			candidates := tree[action].Candidates("do "+action+" ", "", map[string]bool{})
			if !slices.Contains(candidates, "do "+action+" --state-only") {
				t.Errorf("%s %s candidates: %v", name, action, candidates)
			}
		}
	}
}

// A state only unprovision is confirmed for what it does, marking resources
// unprovisioned, not for data lost or services interrupted.
func TestUnprovisionMessages(t *testing.T) {
	if l := unprovisionMessages([]string{"--state-only"}); len(l) != 1 || l[0] != stateOnlyUnprovisionMessage {
		t.Errorf("state only: %v", l)
	}
	if l := unprovisionMessages(nil); !slices.Contains(l, dataLostMessage) {
		t.Errorf("unprovision: %v", l)
	}
}
