package rescontainerocibase

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSession(t *testing.T, script, ready string) *quiesceSession {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	stdinR, stdinW, err := os.Pipe()
	require.NoError(t, err)
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Stdin = stdinR
	return newQuiesceSession(cmd, stdinR, stdinW, cancel, ready, t.Logf)
}

// A session holds until the resume: it reads its input, says it is ready,
// and reads the resume input before its stdin closes.
func TestQuiesceSession(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")
	s := newTestSession(t, `read lock; echo "$lock" >`+out+`; echo quiesced; read unlock; echo "$unlock" >>`+out, "quiesced")
	require.NoError(t, s.start("LOCK"))
	require.NoError(t, s.waitReady(5*time.Second))
	require.NoError(t, s.end("UNLOCK", 5*time.Second))
	b, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "LOCK\nUNLOCK\n", string(b))
}

// A session ending on its own after saying it is ready did not hold the
// writes until the resume: the quiesce or the resume fails, whichever sees
// the end first.
func TestQuiesceSessionEndingBeforeTheResume(t *testing.T) {
	s := newTestSession(t, "echo quiesced; exit 0", "quiesced")
	require.NoError(t, s.start(""))
	if err := s.waitReady(5 * time.Second); err != nil {
		assert.ErrorContains(t, err, "ended before saying")
		return
	}
	require.Eventually(t, func() bool { return len(s.doneC) == 1 }, 5*time.Second, 10*time.Millisecond, "the command exits")
	assert.ErrorContains(t, s.resume("", 5*time.Second), "ended before the resume")
}

// A session holding until the resume resumes cleanly.
func TestQuiesceSessionResume(t *testing.T) {
	s := newTestSession(t, "echo quiesced; read unlock", "quiesced")
	require.NoError(t, s.start(""))
	require.NoError(t, s.waitReady(5*time.Second))
	assert.NoError(t, s.resume("UNLOCK", 5*time.Second))
}

// A command with no ready line runs to its end.
func TestQuiesceOneShot(t *testing.T) {
	s := newTestSession(t, "exit 0", "")
	require.NoError(t, s.start(""))
	assert.NoError(t, s.end("", 5*time.Second))

	s = newTestSession(t, "echo cannot flush >&2; exit 3", "")
	require.NoError(t, s.start(""))
	assert.ErrorContains(t, s.end("", 5*time.Second), "cannot flush")
}

// A session not ready in time, or ending first, fails the quiesce.
func TestQuiesceNotReady(t *testing.T) {
	s := newTestSession(t, "sleep 30", "quiesced")
	require.NoError(t, s.start(""))
	assert.ErrorContains(t, s.waitReady(200*time.Millisecond), "did not say")

	s = newTestSession(t, "echo access denied >&2; exit 1", "quiesced")
	require.NoError(t, s.start(""))
	assert.ErrorContains(t, s.waitReady(5*time.Second), "access denied")
}
