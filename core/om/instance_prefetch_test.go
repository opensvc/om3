package om

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The prefetch is the daemon's to run: the command is not offered, takes a
// resource selector, and no lock flag, as it takes no lock.
func TestInstancePrefetchIsHiddenAndLockFree(t *testing.T) {
	cmd := newCmdObjectInstancePrefetch("svc")
	assert.True(t, cmd.Hidden, "the prefetch command is not offered")
	require.NotNil(t, cmd.Flags().Lookup("rid"), "no --rid flag")
	assert.Nil(t, cmd.Flags().Lookup("waitlock"), "a --waitlock flag")
	assert.Nil(t, cmd.Flags().Lookup("no-lock"), "a --no-lock flag")
}

// The words the daemon execs name the command, from an object path as from
// the svc kind.
func TestInstancePrefetchResolves(t *testing.T) {
	requireResolves(t, []string{"test/svc/s1", "instance", "prefetch", "--rid", "container#1"})
	requireResolves(t, []string{"svc", "instance", "prefetch"})
}
