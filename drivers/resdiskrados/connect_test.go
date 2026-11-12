package resdiskrados

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/command"
)

// A rbd command asking the cluster waits for its monitors the connect timeout
// of its kind, a query less than an action. A command that could not connect
// marks the cluster unreachable, and the next commands asking it fail at once
// rather than wait as long, while the commands acting on the node alone, as
// listing and unmapping the devices, still run.
func TestAClusterNotAnsweringIsNotAskedAgain(t *testing.T) {
	r := &T{}
	t.Cleanup(func() { unreachableClusters.Delete(r.clusterKey()) })

	args, err := r.rbdArgs(mountTimeoutQuery)
	require.NoError(t, err)
	assert.Equal(t, []string{"--client_mount_timeout", "5"}, args)
	args, err = r.rbdArgs(mountTimeoutAction)
	require.NoError(t, err)
	assert.Equal(t, []string{"--client_mount_timeout", "10"}, args)

	cmd := command.New(
		command.WithName("sh"),
		command.WithVarArgs("-c", "echo \"rbd: couldn't connect to the cluster!\" >&2; exit 110"),
		command.WithIgnoredExitCodes(110),
		r.watchConnect(),
	)
	require.NoError(t, cmd.Run())

	_, err = r.rbdArgs(mountTimeoutQuery)
	assert.ErrorContains(t, err, "did not answer a previous command within the connect timeout")
	_, err = r.rbdLocalArgs()
	assert.NoError(t, err, "the commands acting on the node alone still run")
}
