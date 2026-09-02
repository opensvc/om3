package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/api"
)

// withConfiguredHeartbeats pins the heartbeats the node configures, so the
// names are checked against a known list rather than against whatever
// configuration the test host carries.
func withConfiguredHeartbeats(t *testing.T, names ...string) {
	t.Helper()
	saved := configuredHeartbeatNames
	configuredHeartbeatNames = func() ([]string, error) { return names, nil }
	t.Cleanup(func() { configuredHeartbeatNames = saved })
}

func TestHeartbeatStreamName(t *testing.T) {
	withConfiguredHeartbeats(t, "hb#1", "hb#11")
	for _, tc := range []struct {
		name string
		want string
	}{
		{"1.rx", "hb#1.rx"},
		{"1.tx", "hb#1.tx"},
		{"11.rx", "hb#11.rx"},

		// The stream id "om daemon hb status" shows carries the prefix, so
		// a name read there is typed back as it was read.
		{"hb#1.rx", "hb#1.rx"},
	} {
		got, err := heartbeatStreamName(api.InPathHeartbeatName(tc.name))
		require.NoErrorf(t, err, "%s must be accepted", tc.name)
		assert.Equal(t, tc.want, got)
	}
}

func TestHeartbeatStreamNameRefusesWhatNoStreamAnswersTo(t *testing.T) {
	withConfiguredHeartbeats(t, "hb#1", "hb#11")
	// "1.rxx" is the one to refuse loudest: it was accepted, queued, and
	// acted on by nobody. "1" is the heartbeat, not a stream of it, and
	// "2.rx" is a stream of a heartbeat this node does not configure.
	for _, name := range []string{"", "1", "hb#1", "1.rxx", "1.RX", "2.rx", "1.rx.tx"} {
		_, err := heartbeatStreamName(api.InPathHeartbeatName(name))
		require.Errorf(t, err, "%s must not be accepted", name)
		assert.Containsf(t, err.Error(), "1.rx, 1.tx, 11.rx, 11.tx", "the error must name the streams that exist, got %s", err)
	}
}

func TestHeartbeatName(t *testing.T) {
	withConfiguredHeartbeats(t, "hb#1", "hb#11")
	for _, tc := range []struct {
		name string
		want string
	}{
		{"1", "hb#1"},
		{"11", "hb#11"},
		{"hb#1", "hb#1"},
	} {
		got, err := heartbeatName(api.InPathHeartbeatName(tc.name))
		require.NoErrorf(t, err, "%s must be accepted", tc.name)
		assert.Equal(t, tc.want, got)
	}
	for _, name := range []string{"", "2", "1.rx", "hb#"} {
		_, err := heartbeatName(api.InPathHeartbeatName(name))
		require.Errorf(t, err, "%s must not be accepted", name)
		assert.Containsf(t, err.Error(), "expected one of 1, 11", "the error must name the heartbeats that exist, got %s", err)
	}
}

func TestHeartbeatNameOnANodeWithNoHeartbeat(t *testing.T) {
	withConfiguredHeartbeats(t)
	for _, err := range []error{
		second(heartbeatName(api.InPathHeartbeatName("1"))),
		second(heartbeatStreamName(api.InPathHeartbeatName("1.rx"))),
	} {
		require.Error(t, err)
		assert.Contains(t, err.Error(), "configures no heartbeat")
	}
}

func second(_ string, err error) error {
	return err
}
