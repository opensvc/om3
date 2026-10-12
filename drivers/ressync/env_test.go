package ressync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A PRD object running on a node that is not PRD does not sync, as in v2: its
// copies on the PRD nodes are the data to fall back to. The other envs sync
// wherever they run.
func TestAPRDObjectOnANonPRDNodeDoesNotSync(t *testing.T) {
	for _, tc := range []struct {
		objectEnv, nodeEnv string
		skip               bool
	}{
		{"PRD", "PRD", false},
		{"PRD", "DRP", true},
		{"PPRD", "PRD", false},
		{"TST", "PRD", false},
		{"TST", "TST", false},
	} {
		r := T{ObjectEnv: tc.objectEnv, NodeEnv: tc.nodeEnv}
		assert.Equal(t, tc.skip, r.EnvSkipReason() != "", "%s object on a %s node", tc.objectEnv, tc.nodeEnv)
	}
}

// A node that is not PRD sends nothing to a PRD node, whatever the env of the
// object: its data would overwrite the production one there.
func TestANonPRDNodeDoesNotSyncToAPRDNode(t *testing.T) {
	for _, tc := range []struct {
		localEnv, peerEnv string
		refused           bool
	}{
		{"TST", "PRD", true},
		{"DRP", "PRD", true},
		{"PRD", "PRD", false},
		{"PRD", "DRP", false},
		{"TST", "TST", false},
	} {
		assert.Equal(t, tc.refused, peerEnvRefusal(tc.localEnv, tc.peerEnv) != nil, "from a %s node to a %s node", tc.localEnv, tc.peerEnv)
	}
}
