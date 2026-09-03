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

func TestHeartbeatStreamNames(t *testing.T) {
	withConfiguredHeartbeats(t, "hb#1", "hb#11")
	for _, tc := range []struct {
		name string
		want []string
	}{
		{"1.rx", []string{"hb#1.rx"}},
		{"1.tx", []string{"hb#1.tx"}},
		{"11.rx", []string{"hb#11.rx"}},

		// The stream id "om daemon hb ls" shows carries the prefix, so
		// a name read there is typed back as it was read.
		{"hb#1.rx", []string{"hb#1.rx"}},

		// A heartbeat is both of its streams: naming it is naming them,
		// which the caller had to spell out.
		{"1", []string{"hb#1.rx", "hb#1.tx"}},
		{"hb#11", []string{"hb#11.rx", "hb#11.tx"}},
	} {
		got, err := heartbeatStreamNames(api.InPathHeartbeatName(tc.name))
		require.NoErrorf(t, err, "%s must be accepted", tc.name)
		assert.Equal(t, tc.want, got)
	}
}

func TestHeartbeatStreamNamesRefusesWhatNoStreamAnswersTo(t *testing.T) {
	withConfiguredHeartbeats(t, "hb#1", "hb#11")
	// "1.rxx" is the one to refuse loudest: it was accepted, queued, and
	// acted on by nobody. "2.rx" is a stream of a heartbeat this node does
	// not configure, and "2" a heartbeat it does not configure at all.
	for _, name := range []string{"", "1.rxx", "1.RX", "2.rx", "2", "1.rx.tx"} {
		_, err := heartbeatStreamNames(api.InPathHeartbeatName(name))
		require.Errorf(t, err, "%s must not be accepted", name)
		assert.Containsf(t, err.Error(), "1.rx, 1.tx, 11.rx, 11.tx", "the error must name the streams that exist, got %s", err)
		assert.Containsf(t, err.Error(), "or 1, 11 for both streams", "the error must say a heartbeat names both, got %s", err)
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
		secondOfSlice(heartbeatStreamNames(api.InPathHeartbeatName("1.rx"))),
	} {
		require.Error(t, err)
		assert.Contains(t, err.Error(), "configures no heartbeat")
	}
}

func second(_ string, err error) error {
	return err
}

func secondOfSlice(_ []string, err error) error {
	return err
}
