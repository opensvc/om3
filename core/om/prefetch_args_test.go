package om

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/prefetch"
)

// The daemon execs om with the argv prefetch.CmdArgs builds, which nothing
// but the command tree checks: an argv naming no command has om print a
// help text and exit 0, read as a prefetch that recorded nothing.
func TestPrefetchCmdArgsResolve(t *testing.T) {
	p, err := naming.ParsePath("test/svc/s1")
	require.NoError(t, err)
	requireResolves(t, prefetch.CmdArgs(p, "container#1"))
}
