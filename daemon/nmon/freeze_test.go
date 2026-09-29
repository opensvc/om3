package nmon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/freeze"
	"github.com/opensvc/om3/v3/core/node"
)

// A node back from down adopts a freeze of the cluster it missed, and no
// other: a peer frozen alone, or by the daemon on its own, stays the only
// one frozen, and a freeze made before the node went down is not one it
// missed.
func TestClusterFreezeMissedBy(t *testing.T) {
	lastShutdownAt := time.Now().Add(-time.Minute)
	after := lastShutdownAt.Add(10 * time.Second)
	before := lastShutdownAt.Add(-time.Hour)

	for _, tc := range []struct {
		name string
		peer node.Status
		want []string
	}{
		{"not frozen", node.Status{}, nil},
		{"cluster frozen while down", node.Status{FrozenAt: after, FrozenScope: freeze.ScopeCluster}, []string{"n2"}},
		{"cluster frozen before going down", node.Status{FrozenAt: before, FrozenScope: freeze.ScopeCluster}, nil},
		{"frozen alone while down", node.Status{FrozenAt: after, FrozenScope: freeze.ScopeNode}, nil},
		{"frozen by an older agent while down", node.Status{FrozenAt: after}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := clusterFreezeMissedBy(map[string]node.Status{"n2": tc.peer}, lastShutdownAt)
			require.Equal(t, tc.want, got)
		})
	}
}
