package om

import (
	"os"
	"os/exec"
	"testing"

	"github.com/opensvc/om3/v3/testhelper"
	"github.com/stretchr/testify/require"
)

// TestIgnoreNotFoundFlag tests the --ignore-not-found flag functionality
func TestIgnoreNotFoundFlag(t *testing.T) {
	env := testhelper.Setup(t)

	t.Run("om svc instance stop without --ignore-not-found should fail with non-existent objects", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "nonexistent/svc/test", "instance", "stop")
		cmd.Env = append(cmd.Env, "OSVC_ROOT_PATH="+env.Root, "GO_TEST_MODE=off")
		cmd.Env = append(cmd.Env, os.Environ()...)
		output, err := cmd.CombinedOutput()
		require.Error(t, err, "Command should fail without --ignore-not-found flag")
		require.Contains(t, string(output), "object not found", "Should contain object not found error")
		if exitErr, ok := err.(*exec.ExitError); ok {
			require.Equal(t, 2, exitErr.ExitCode(), "Exit code should be 2 for ObjectNotFound")
		}
	})

	t.Run("om svc instance stop with --ignore-not-found should succeed with non-existent objects", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "nonexistant/svc/test", "instance", "stop", "--ignore-not-found")
		cmd.Env = append(cmd.Env, "OSVC_ROOT_PATH="+env.Root, "GO_TEST_MODE=off")
		cmd.Env = append(cmd.Env, os.Environ()...)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Command should succeed with --ignore-not-found flag")
		require.Empty(t, output, "Should have no output for non-existent objects")
	})

	t.Run("om cfg config get without --ignore-not-found should succeed with non-existent objects", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "nonexistant/cfg/test", "config", "get", "--kw", "id", "--local")
		cmd.Env = append(cmd.Env, "OSVC_ROOT_PATH="+env.Root, "GO_TEST_MODE=off")
		cmd.Env = append(cmd.Env, os.Environ()...)
		output, err := cmd.CombinedOutput()
		require.Error(t, err, "Command should fail without --ignore-not-found flag")
		require.Contains(t, string(output), "object not found", "Should contain object not found error")
		if exitErr, ok := err.(*exec.ExitError); ok {
			require.Equal(t, 2, exitErr.ExitCode(), "Exit code should be 2 for ObjectNotFound")
		}
	})

	t.Run("om cfg config get with --ignore-not-found should succeed with non-existent objects", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "nonexistant/cfg/test", "config", "get", "--kw", "id", "--local", "--ignore-not-found")
		cmd.Env = append(cmd.Env, "OSVC_ROOT_PATH="+env.Root, "GO_TEST_MODE=off")
		cmd.Env = append(cmd.Env, os.Environ()...)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Command should succeed with --ignore-not-found flag")
		require.Empty(t, output, "Should have no output for non-existent objects")
	})
}
