package resource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequiresSync(t *testing.T) {
	r := T{SyncRequires: "fs#2(up)"}
	for _, action := range []string{"sync_update", "sync_full"} {
		_, ok := r.Requires(action).Requirements()["fs#2"]
		require.True(t, ok, action)
	}
	require.Empty(t, r.Requires("start").Requirements())
}
