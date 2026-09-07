package rescontainerpodman

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/drivers/rescontainerocibase"
)

// Test_ExecBaseArgs pins that no base argument is passed to podman.
//
// The driver used to pass "--cni-config-dir", from a node keyword with a
// default, so every podman command carried it. Podman never read it: the
// netns keyword resolves to a host, a private or another container's
// namespace, and the ip drivers configure the addresses from outside. Podman
// 5 dropped the cni backend and the flag with it, where it stopped being
// inert and became an unknown flag.
func Test_ExecBaseArgs(t *testing.T) {
	d := &T{}

	if err := d.Configure(); err != nil {
		require.NoError(t, err)
	}

	baseArgs := d.
		Executer().(rescontainerocibase.ExecutorArgserGetter).
		ExecutorArgser().(rescontainerocibase.ExecutorBaseArgser).
		ExecBaseArgs()

	require.Emptyf(t, baseArgs, "podman is passed no base argument, got: %s", baseArgs)
}
