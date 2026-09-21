package command

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A service manager addresses some variables to the service it starts, and to
// that service alone. A command the service runs is not that service: a
// container runtime finding NOTIFY_SOCKET waits for the container to report
// itself ready, which a container that is not a service never does.
func TestACommandDoesNotInheritWhatSystemdSaidToUsAlone(t *testing.T) {
	for _, name := range serviceManagerEnv {
		t.Setenv(name, "/run/systemd/notify")
	}
	t.Setenv("OSVC_KEPT", "kept")

	cmd := New(
		WithName("env"),
		WithBufferedStdout(),
		WithEnv([]string{"OSVC_ADDED=added"}),
	)
	require.NoError(t, cmd.Run())
	out := string(cmd.Stdout())

	for _, name := range serviceManagerEnv {
		assert.NotContainsf(t, out, name+"=", "%s reached the command", name)
	}
	assert.Contains(t, out, "OSVC_KEPT=kept", "the rest of the environment is passed on")
	assert.Contains(t, out, "OSVC_ADDED=added", "and so is what the caller added")

	// The daemon goes on notifying for as long as it runs, so what it was
	// given stays where it is.
	assert.NotEmpty(t, os.Getenv("NOTIFY_SOCKET"))
	assert.True(t, strings.Contains(strings.Join(os.Environ(), " "), "NOTIFY_SOCKET="))
}
