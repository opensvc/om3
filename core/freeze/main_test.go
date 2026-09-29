package freeze

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A frozen flag says the scope of the freeze that raised it, and keeps it: a
// freeze asked again, of another scope, does not change what was decided.
func TestFrozenScope(t *testing.T) {
	d := t.TempDir()

	local := filepath.Join(d, "local", "frozen")
	require.Equal(t, Scope(""), ScopeOf(local, ScopeCluster, ScopeNode), "not frozen")
	require.NoError(t, Freeze(local))
	require.Equal(t, ScopeNode, ScopeOf(local, ScopeCluster, ScopeNode))
	require.NoError(t, FreezeScope(local, ScopeCluster))
	require.Equal(t, ScopeNode, ScopeOf(local, ScopeCluster, ScopeNode), "a flag raised keeps its scope")

	wide := filepath.Join(d, "wide", "frozen")
	require.NoError(t, FreezeScope(wide, ScopeCluster))
	require.Equal(t, ScopeCluster, ScopeOf(wide, ScopeCluster, ScopeNode))
	require.Equal(t, ScopeInstance, ScopeOf(wide, ScopeObject, ScopeInstance), "a scope foreign to the flag reads local")

	require.NoError(t, Unfreeze(wide))
	require.Equal(t, Scope(""), ScopeOf(wide, ScopeCluster, ScopeNode))
}
