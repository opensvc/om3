package commoncmd

import (
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
)

func heartbeatRow(t *testing.T, state string, isBeating, isSingleNode bool) map[string]any {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	savedNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = savedNoColor })

	entry := daemonsubsystem.HeartbeatStreamPeerStatusTableEntry{
		HeartbeatStreamPeerStatus: daemonsubsystem.HeartbeatStreamPeerStatus{
			IsBeating: isBeating,
		},
		IsSingleNode: isSingleNode,
	}
	entry.Status.State = state
	return heartbeatStreamRow{entry}.Unstructured()
}

// TestHeartbeatStreamRowDrawsTheIcons pins that the listing composes the
// two icon columns its default output names, from the values the daemon
// type carries.
func TestHeartbeatStreamRowDrawsTheIcons(t *testing.T) {
	for _, tc := range []struct {
		state        string
		isBeating    bool
		isSingleNode bool
		stateIcon    string
		beatingIcon  string
	}{
		{"running", true, false, "O", "O"},
		{"stopped", false, false, "X", "X"},
		{"failed", false, false, "X", "X"},
		{"warning", true, false, "!", "O"},
		{"", false, false, "?", "X"},

		// A single node has no peer to beat with, and is not stale for it.
		{"running", false, true, "O", "O"},
	} {
		m := heartbeatRow(t, tc.state, tc.isBeating, tc.isSingleNode)
		stateIcon, ok := m["state_icon"].(string)
		require.Truef(t, ok, "state %q has no state_icon", tc.state)
		beatingIcon, ok := m["beating_icon"].(string)
		require.Truef(t, ok, "state %q has no beating_icon", tc.state)

		assert.Equalf(t, tc.stateIcon, stripANSI(stateIcon), "state icon of %q", tc.state)
		assert.Equalf(t, tc.beatingIcon, stripANSI(beatingIcon), "beating icon of %q", tc.state)
		assert.Containsf(t, stateIcon, "\x1b[", "the state icon of %q must be colored", tc.state)
		assert.Containsf(t, beatingIcon, "\x1b[", "the beating icon of %q must be colored", tc.state)
	}
}

// TestHeartbeatStreamRowKeepsWhatTheEntryCarries pins that the row adds to
// the entry rather than replacing it: the other columns of the default
// output are still resolved.
func TestHeartbeatStreamRowKeepsWhatTheEntryCarries(t *testing.T) {
	m := heartbeatRow(t, "running", true, false)
	for _, key := range []string{"id", "node", "peer", "type", "desc", "changed_at", "state", "is_beating"} {
		assert.Containsf(t, m, key, "%s is named by the default output", key)
	}
}

func stripANSI(s string) string {
	for {
		i := strings.Index(s, "\x1b[")
		if i < 0 {
			return s
		}
		j := strings.IndexByte(s[i:], 'm')
		if j < 0 {
			return s
		}
		s = s[:i] + s[i+j+1:]
	}
}
