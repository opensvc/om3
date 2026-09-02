package daemoncmd

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

// TestRotateCrashFile covers what happens to the report of a crash when
// the daemon that replaces the crashed one starts: it must survive the
// truncation, and nothing must be kept that is not a report.
func TestRotateCrashFile(t *testing.T) {
	t.Run("a report is moved out of the way", func(t *testing.T) {
		dir := t.TempDir()
		filename := filepath.Join(dir, "om.stack")
		require.NoError(t, os.WriteFile(filename, []byte("panic: boom\n"), 0644))

		require.NoError(t, rotateCrashFile(filename))

		assert.NoFileExists(t, filename, "the file is left for the new daemon to create")
		b, err := os.ReadFile(filename + ".1")
		require.NoError(t, err)
		assert.Equal(t, "panic: boom\n", string(b))
	})

	t.Run("an empty file is not a report", func(t *testing.T) {
		dir := t.TempDir()
		filename := filepath.Join(dir, "om.stack")
		require.NoError(t, os.WriteFile(filename, nil, 0644))

		require.NoError(t, rotateCrashFile(filename))

		assert.FileExists(t, filename, "a daemon that did not crash leaves an empty file")
		assert.NoFileExists(t, filename+".1")
	})

	t.Run("a missing file is not an error", func(t *testing.T) {
		filename := filepath.Join(t.TempDir(), "om.stack")
		assert.NoError(t, rotateCrashFile(filename))
		assert.NoFileExists(t, filename+".1")
	})

	t.Run("only the previous report is kept", func(t *testing.T) {
		dir := t.TempDir()
		filename := filepath.Join(dir, "om.stack")
		require.NoError(t, os.WriteFile(filename+".1", []byte("older\n"), 0644))
		require.NoError(t, os.WriteFile(filename, []byte("newer\n"), 0644))

		require.NoError(t, rotateCrashFile(filename))

		b, err := os.ReadFile(filename + ".1")
		require.NoError(t, err)
		assert.Equal(t, "newer\n", string(b), "the report of the last crash replaces the one before it")
	})
}

// TestSetupCrashReport covers the install: the runtime is given a file
// to write its report to, and the report of the previous crash is kept.
func TestSetupCrashReport(t *testing.T) {
	previous := rawconfig.Paths
	rawconfig.Paths.Var = t.TempDir()
	t.Cleanup(func() {
		rawconfig.Paths = previous
		// Leave the runtime writing to stderr alone, rather than to a
		// file this test is about to remove.
		_ = debug.SetCrashOutput(nil, debug.CrashOptions{})
	})

	filename := filepath.Join(rawconfig.Paths.Var, crashFileBasename)
	require.NoError(t, os.WriteFile(filename, []byte("panic: the previous daemon\n"), 0644))

	setupCrashReport()

	assert.FileExists(t, filename, "the runtime needs the file to exist to write to it")
	info, err := os.Stat(filename)
	require.NoError(t, err)
	assert.Zero(t, info.Size(), "this daemon has not crashed yet")

	b, err := os.ReadFile(filename + ".1")
	require.NoError(t, err)
	assert.Equal(t, "panic: the previous daemon\n", string(b))
}
