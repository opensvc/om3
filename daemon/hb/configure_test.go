package hb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/hbcfg"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/hbcache"
)

// refusedConfer is a heartbeat whose driver refuses its configuration.
type refusedConfer struct {
	hbcfg.T
	err error
}

func (t *refusedConfer) Configure(context.Context) error {
	return t.err
}

// streams returns the heartbeat streams the listing shows, by id, once the
// controller published them as want says.
func streams(t *testing.T, want func(map[string]daemonsubsystem.HeartbeatStream) bool) map[string]daemonsubsystem.HeartbeatStream {
	t.Helper()
	var m map[string]daemonsubsystem.HeartbeatStream
	require.Eventually(t, func() bool {
		l, _ := hbcache.Heartbeats()
		m = make(map[string]daemonsubsystem.HeartbeatStream)
		for _, s := range l {
			m[s.ID] = s
		}
		return want(m)
	}, 5*time.Second, 50*time.Millisecond)
	return m
}

// TestAHeartbeatFailingToConfigureIsListedFailed pins that a heartbeat whose
// driver refuses its configuration is listed, failed, with the error as an
// alert: it was not listed at all, and nothing said why it did not run. Its
// streams go with its section.
func TestAHeartbeatFailingToConfigureIsListedFailed(t *testing.T) {
	h := newTestManager(t)
	ctx := h.msgToTxCtx

	refused := &refusedConfer{err: errors.New("the IPv6 group ff02::1:4141 needs intf, on node n1")}
	refused.SetName("hb#v6")
	refused.SetDriver("multicast")
	hb := confer{Confer: refused, err: refused.Configure(ctx)}

	err := h.startHb(hb)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "needs intf")

	m := streams(t, func(m map[string]daemonsubsystem.HeartbeatStream) bool {
		_, tx := m["hb#v6.tx"]
		_, rx := m["hb#v6.rx"]
		return tx && rx
	})
	for _, id := range []string{"hb#v6.tx", "hb#v6.rx"} {
		s := m[id]
		assert.Equal(t, "failed", s.State, id)
		assert.Equal(t, "multicast", s.Type, id)
		assert.Equal(t, []daemonsubsystem.Alert{{Severity: "error", Message: "the IPv6 group ff02::1:4141 needs intf, on node n1"}}, s.Alerts, id)
	}
	assert.Equal(t, []string{"hb#v6"}, h.knownRids(), "a failed heartbeat is known, to go with its section")

	// a retry failing again replaces the streams, rather than adding some
	require.Error(t, h.startHb(hb))
	assert.Len(t, h.failed, 2)

	// the section removed
	require.NoError(t, h.stopHbRid("hb#v6"))
	streams(t, func(m map[string]daemonsubsystem.HeartbeatStream) bool {
		_, tx := m["hb#v6.tx"]
		_, rx := m["hb#v6.rx"]
		return !tx && !rx
	})
	assert.Empty(t, h.knownRids())
}
