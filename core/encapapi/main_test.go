package encapapi

import (
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/xerrors"
)

type fakeReader struct{ events []*event.Event }

func (t *fakeReader) Read() (*event.Event, error) {
	if len(t.events) == 0 {
		return nil, io.EOF
	}
	ev := t.events[0]
	t.events = t.events[1:]
	return ev, nil
}

func execEvent(t *testing.T, kind string, exec uuid.UUID, msg string, exitCode int) *event.Event {
	t.Helper()
	b, err := json.Marshal(map[string]any{"exec_id": exec, "error": msg, "exit_code": exitCode})
	require.NoError(t, err)
	return &event.Event{Kind: kind, Data: b}
}

// The session of the master is shared by all the execs it asks of the guest:
// the end of another one is not the end of the one waited for.
func TestWaitExecWaitsForItsOwnExec(t *testing.T) {
	mine, other := uuid.New(), uuid.New()

	assert.NoError(t, waitExec(&fakeReader{events: []*event.Event{
		execEvent(t, "ExecFailed", other, "not mine", 1),
		execEvent(t, "ExecSuccess", mine, "", 0),
	}}, mine))

	err := waitExec(&fakeReader{events: []*event.Event{
		execEvent(t, "ExecSuccess", other, "", 0),
		execEvent(t, "ExecFailed", mine, "exit code 1", 1),
	}}, mine)
	assert.EqualError(t, err, "exit code 1")

	err = waitExec(&fakeReader{events: []*event.Event{
		execEvent(t, "ExecFailed", mine, "exit code 2", xerrors.ExitCodeObjectNotFound),
	}}, mine)
	assert.True(t, errors.Is(err, ErrNotFound), "%v", err)

	err = waitExec(&fakeReader{}, mine)
	assert.True(t, errors.Is(err, io.EOF), "%v", err)
}

func TestParseStates(t *testing.T) {
	p, err := naming.ParsePath("ns1/svc/foo")
	require.NoError(t, err)
	states, err := New(nil, "vm1", p).parseStates([]byte(`{"data":{"config":{"csum":"abc"},"status":{"avail":"up"}}}`))
	require.NoError(t, err)
	assert.Equal(t, "vm1", states.Node.Name)
	assert.Equal(t, p, states.Path)
	assert.Equal(t, "up", states.Status.Avail.String())

	_, err = New(nil, "vm1", p).parseStates([]byte(`{`))
	assert.Error(t, err)
}

func TestSupports(t *testing.T) {
	v := &T{}
	for _, s := range []string{"provision", "shutdown", "start", "startstandby", "stop", "unprovision"} {
		assert.True(t, v.Supports(s), s)
	}
	assert.False(t, v.Supports("sync_update"))
}
