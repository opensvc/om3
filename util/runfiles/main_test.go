package runfiles

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/bootid"
	"github.com/opensvc/om3/v3/util/proc"
)

func TestRunFiles(t *testing.T) {
	d := Dir{
		Path: t.TempDir(),
	}

	t.Run("Count", func(t *testing.T) {
		n, err := d.Count()
		assert.NoError(t, err)
		assert.Zero(t, n)
	})

	t.Run("Create", func(t *testing.T) {
		content := "foo"
		err := d.Create([]byte(content))
		assert.NoError(t, err)
		l, err := d.List()
		require.NoError(t, err)
		require.Len(t, l, 1)
		assert.Equal(t, content, string(l[0].Content), "the content read is the one written, without the identity line")
		assert.Equal(t, os.Getpid(), l[0].PID)
	})

	t.Run("CountAndClean", func(t *testing.T) {
		n, err := d.CountAndClean()
		assert.NoError(t, err)
		assert.Equal(t, 1, n)
	})

	t.Run("CreateStale", func(t *testing.T) {
		content := "foo"
		err := d.create(0, []byte(content))
		assert.NoError(t, err)
	})

	t.Run("CountAndCleanWithStale", func(t *testing.T) {
		n, err := d.CountAndClean()
		assert.NoError(t, err)
		assert.Equal(t, 1, n)
	})

	t.Run("Remove", func(t *testing.T) {
		err := d.Remove()
		assert.NoError(t, err)
	})
}

// A run is alive while its writer runs, whatever the time of the directory
// of the process in /proc, which the kernel recreates: a run judged stale
// was cleaned up, and another admitted beyond max_parallel.
func TestARunIsAliveWhileItsWriterRuns(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	require.NoError(t, cmd.Start())
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	pid := cmd.Process.Pid

	d := Dir{Path: t.TempDir()}
	require.NoError(t, d.create(pid, []byte("session")))
	// The run file older than the directory of the process.
	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(d.filename(pid), old, old))

	n, err := d.CountAndClean()
	require.NoError(t, err)
	assert.Equal(t, 1, n, "the run of the process running is alive")

	// Another process given the pid: another start time.
	start, err := proc.New(pid).StartTime()
	require.NoError(t, err)
	content := fmt.Sprintf("%s%d %s\n%s", headerPrefix, start+1, bootid.Get(), "session")
	require.NoError(t, os.WriteFile(d.filename(pid), []byte(content), 0600))
	n, err = d.CountAndClean()
	require.NoError(t, err)
	assert.Zero(t, n, "the run of another process with the pid is stale")
	_, err = os.Stat(d.filename(pid))
	assert.ErrorIs(t, err, os.ErrNotExist, "and cleaned up")
}

// A run file of an older agent, naming no writer, is judged by the time of
// the directory of the process, as before.
func TestARunFileNamingNoWriterIsJudgedByItsTime(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	require.NoError(t, cmd.Start())
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	pid := cmd.Process.Pid

	d := Dir{Path: t.TempDir()}
	path := filepath.Join(d.Path, fmt.Sprint(pid))
	require.NoError(t, os.WriteFile(path, []byte("session"), 0600))
	// The run file newer than the directory of the process.
	later := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(path, later, later))
	l, err := d.List()
	require.NoError(t, err)
	require.Len(t, l, 1)
	assert.Equal(t, "session", string(l[0].Content))

	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(path, old, old))
	n, err := d.Count()
	require.NoError(t, err)
	assert.Zero(t, n)
}
