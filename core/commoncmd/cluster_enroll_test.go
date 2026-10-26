package commoncmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

func TestCmdClusterEnrollToken(t *testing.T) {
	writeTokenFile := func(t *testing.T, s string) string {
		t.Helper()
		name := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(name, []byte(s), 0600))
		return name
	}

	t.Run("reads the token from the file --token names, trimmed", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "")
		c := CmdClusterEnroll{TokenFile: writeTokenFile(t, "  from-file\n")}
		got, err := c.token()
		require.NoError(t, err)
		assert.Equal(t, "from-file", got)
	})

	t.Run("prefers --token over the environment", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "from-env")
		c := CmdClusterEnroll{TokenFile: writeTokenFile(t, "from-file")}
		got, err := c.token()
		require.NoError(t, err)
		assert.Equal(t, "from-file", got)
	})

	t.Run("falls back to the environment", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "from-env")
		c := CmdClusterEnroll{}
		got, err := c.token()
		require.NoError(t, err)
		assert.Equal(t, "from-env", got)
	})

	t.Run("refuses an empty --token file", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "")
		c := CmdClusterEnroll{TokenFile: writeTokenFile(t, "\n  \n")}
		_, err := c.token()
		assert.ErrorIs(t, err, ErrFlagInvalid)
	})

	t.Run("refuses an unreadable --token file", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "")
		c := CmdClusterEnroll{TokenFile: filepath.Join(t.TempDir(), "absent")}
		_, err := c.token()
		assert.ErrorIs(t, err, ErrFlagInvalid)
	})

	t.Run("refuses no token at all", func(t *testing.T) {
		t.Setenv(env.JoinTokenVar, "")
		c := CmdClusterEnroll{}
		_, err := c.token()
		assert.ErrorIs(t, err, ErrFlagInvalid)
	})
}

// fakeEventReader serves the events of msgs, then io.EOF.
type fakeEventReader struct {
	msgs []any
}

func (r *fakeEventReader) Read() (*event.Event, error) {
	if len(r.msgs) == 0 {
		return nil, io.EOF
	}
	m := r.msgs[0]
	r.msgs = r.msgs[1:]
	return event.ToEvent(m, 0), nil
}

func (r *fakeEventReader) Close() error { return nil }

// captureStdout returns what f writes on the standard output.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()
	f()
	require.NoError(t, w.Close())
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(b)
}

// TestCmdClusterEnrollWaitCollector pins that the enroll wait reports the
// collector registration of the enrolled node from its first collector
// state, whether it comes before or after the node is alive.
func TestCmdClusterEnrollWaitCollector(t *testing.T) {
	collectorUpdated := func(node, state string) *msgbus.DaemonCollectorUpdated {
		m := &msgbus.DaemonCollectorUpdated{Node: node}
		m.Value.State = state
		return m
	}
	for name, tc := range map[string]struct {
		msgs []any
		want string
		not  string
	}{
		"registered, state after alive": {
			msgs: []any{
				&msgbus.NodeAlive{Node: "node2"},
				collectorUpdated("node2", ""),
				collectorUpdated("node3", "warning"),
				collectorUpdated("node2", "speaker-candidate"),
			},
			want: "Node node2 is alive\nNode node2 is registered on the collector\n",
		},
		"registered, state before alive": {
			msgs: []any{
				collectorUpdated("node2", "speaker"),
				&msgbus.NodeAlive{Node: "node2"},
			},
			want: "Node node2 is alive\nNode node2 is registered on the collector\n",
		},
		"not registered": {
			msgs: []any{
				&msgbus.NodeAlive{Node: "node2"},
				collectorUpdated("node2", "warning"),
			},
			want: "Node node2 is not registered on the collector (state warning): register it with 'om node register'\n",
		},
		"no collector": {
			msgs: []any{
				&msgbus.NodeAlive{Node: "node2"},
				collectorUpdated("node2", "disabled"),
			},
			want: "Node node2 is alive\n",
			not:  "collector",
		},
		"no collector state": {
			msgs: []any{
				&msgbus.NodeAlive{Node: "node2"},
			},
			want: "Node node2 collector state not received",
		},
	} {
		t.Run(name, func(t *testing.T) {
			var err error
			out := captureStdout(t, func() {
				err = (&CmdClusterEnroll{}).waitResult(context.Background(), &fakeEventReader{msgs: tc.msgs}, "node2")
			})
			require.NoError(t, err)
			assert.Contains(t, out, tc.want)
			if tc.not != "" {
				assert.NotContains(t, out, tc.not)
			}
		})
	}
}
