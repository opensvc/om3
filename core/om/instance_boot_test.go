package om

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInstanceBootWaitsForTheLock verifies the boot the daemon runs waits
// for the object lock as the other actions do: its lock timeout unset, it
// tried the lock once, and failed when a status evaluation held it.
func TestInstanceBootWaitsForTheLock(t *testing.T) {
	cmd := newCmdObjectInstanceBoot("svc")
	f := cmd.Flags().Lookup("waitlock")
	require.NotNil(t, f, "no --waitlock flag")
	require.Equal(t, "30s", f.DefValue)
	require.NotNil(t, cmd.Flags().Lookup("no-lock"), "no --no-lock flag")
}
