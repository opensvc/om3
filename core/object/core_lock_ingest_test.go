//go:build linux

package object

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
)

// TestHelperHoldLock is not a test: run by TestAnIngestHoldingTheLockIsKnown
// in a process of its own, it holds the lock at OM3_TEST_LOCK with the intent
// OM3_TEST_INTENT until its standard input closes, an fcntl lock being the
// process's, which a probe of the same process takes.
func TestHelperHoldLock(t *testing.T) {
	p := os.Getenv("OM3_TEST_LOCK")
	if p == "" {
		t.Skip("helper")
	}
	lock := flock.New(p, "helper", fcntllock.New)
	require.NoError(t, lock.Lock(time.Second, os.Getenv("OM3_TEST_INTENT")))
	os.Stdout.WriteString("locked\n")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

func holdLock(t *testing.T, p, intent string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperHoldLock$")
	// The test binary of the package runs the om command when
	// GO_TEST_MODE is off, as testhelper.Main sets it for the tests:
	// the helper is run as a test.
	env := make([]string, 0, len(os.Environ())+2)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GO_TEST_MODE=") {
			env = append(env, e)
		}
	}
	cmd.Env = append(env, "OM3_TEST_LOCK="+p, "OM3_TEST_INTENT="+intent)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = stdin.Close(); _ = cmd.Wait() })
	// The test binary may say other things first.
	r := bufio.NewReader(stdout)
	for {
		line, err := r.ReadString('\n')
		require.NoError(t, err, "the helper exited before it held the lock")
		if line == "locked\n" {
			break
		}
	}
}

// The ingest of the resource files the daemon fetched holds the object lock
// as sync_ingest: an action meeting it knows it, and waits for it rather than
// fail on the time a lock is usually waited for. A lock held for anything
// else is no ingest.
func TestAnIngestHoldingTheLockIsKnown(t *testing.T) {
	dir := t.TempDir()
	ingest := filepath.Join(dir, "ingest")
	holdLock(t, ingest, actioncontext.SyncIngest.Name)
	holder := (&core{}).ingestHoldingLock(flock.New(ingest, "probe", fcntllock.New))
	require.NotNil(t, holder, "the ingest holds the lock")
	assert.NotZero(t, holder.PID)

	other := filepath.Join(dir, "other")
	holdLock(t, other, actioncontext.Start.Name)
	assert.Nil(t, (&core{}).ingestHoldingLock(flock.New(other, "probe", fcntllock.New)), "a start holds the lock")

	free := filepath.Join(dir, "free")
	assert.Nil(t, (&core{}).ingestHoldingLock(flock.New(free, "probe", fcntllock.New)), "nothing holds the lock")
}
