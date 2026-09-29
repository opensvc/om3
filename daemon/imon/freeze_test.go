package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/freeze"
	"github.com/opensvc/om3/v3/core/instance"
)

// An instance back from down adopts a freeze of the object it missed, and no
// other: a peer instance frozen alone, or by an adoption of its own, stays
// the only one frozen.
func TestObjectFreezeMissed(t *testing.T) {
	leftAt := time.Now().Add(-time.Minute)
	rejoinedAt := time.Now()
	during := leftAt.Add(10 * time.Second)

	for _, tc := range []struct {
		name string
		peer instance.Status
		want bool
	}{
		{"not frozen", instance.Status{}, false},
		{"object frozen while down", instance.Status{FrozenAt: during, FrozenScope: freeze.ScopeObject}, true},
		{"object frozen before going down", instance.Status{FrozenAt: leftAt.Add(-time.Hour), FrozenScope: freeze.ScopeObject}, false},
		{"object frozen after rejoining", instance.Status{FrozenAt: rejoinedAt.Add(time.Second), FrozenScope: freeze.ScopeObject}, false},
		{"instance frozen alone while down", instance.Status{FrozenAt: during, FrozenScope: freeze.ScopeInstance}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statuses := map[string]instance.Status{
				"n1": {FrozenAt: during, FrozenScope: freeze.ScopeObject},
				"n2": tc.peer,
			}
			peer, _, ok := objectFreezeMissed(statuses, "n1", leftAt, rejoinedAt)
			require.Equal(t, tc.want, ok)
			if ok {
				require.Equal(t, "n2", peer)
			}
		})
	}
}
