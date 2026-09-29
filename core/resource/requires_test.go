package resource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequiresUpdate(t *testing.T) {
	r := T{UpdateRequires: "fs#2(up)"}
	for _, action := range []string{"update", "full"} {
		_, ok := r.Requires(action).Requirements()["fs#2"]
		require.True(t, ok, action)
	}
	require.Empty(t, r.Requires("start").Requirements())
}
