package daemondata

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type nopPublisher struct{}

func (nopPublisher) Pub(pubsub.Messager, ...pubsub.Label) {}

// TestSetNextMsgType pins when a node sends full messages, and when it
// switches to patch ones.
func TestSetNextMsgType(t *testing.T) {
	newData := func(msgType string, gen uint64, peerGens map[string]uint64) *data {
		d := &data{
			localNode:     "n1",
			gen:           gen,
			hbMessageType: msgType,
			hbMsgType:     map[string]string{},
			hbGens:        map[string]node.Gen{"n1": {"n1": gen}},
			clusterNodes:  map[string]struct{}{"n1": {}},
			log:           plog.NewDefaultLogger(),
			publisher:     nopPublisher{},
		}
		for peer, g := range peerGens {
			d.clusterNodes[peer] = struct{}{}
			d.hbGens[peer] = node.Gen{"n1": g, peer: 1}
		}
		return d
	}
	for name, tc := range map[string]struct {
		msgType  string
		gen      uint64
		peerGens map[string]uint64
		want     string
	}{
		"init":                               {msgType: "undef", gen: 1, want: "ping"},
		"no peer message yet":                {msgType: "ping", gen: 1, want: "ping"},
		"first type after ping is full":      {msgType: "ping", gen: 1, peerGens: map[string]uint64{"n2": 1}, want: "full"},
		"a peer applied no gen of this node": {msgType: "patch", gen: 10, peerGens: map[string]uint64{"n2": 10, "n3": 0}, want: "full"},
		"stay in full for a peer needing it": {msgType: "full", gen: 10, peerGens: map[string]uint64{"n2": 10, "n3": 0}, want: "full"},
		"peers applied the current gen":      {msgType: "full", gen: 10, peerGens: map[string]uint64{"n2": 10, "n3": 10}, want: "patch"},
		"a peer applied an older gen":        {msgType: "full", gen: 36, peerGens: map[string]uint64{"n2": 35, "n3": 30}, want: "patch"},
		"stay in patch while peers catch up": {msgType: "patch", gen: 36, peerGens: map[string]uint64{"n2": 35, "n3": 30}, want: "patch"},
	} {
		t.Run(name, func(t *testing.T) {
			d := newData(tc.msgType, tc.gen, tc.peerGens)
			d.setNextMsgType()
			assert.Equal(t, tc.want, d.hbMessageType)
		})
	}
}

// TestAcceptPeerRun pins the detection of a peer restart by its daemon run id:
// it asks the peer a full message even when the gen of the new run does not
// go back below the one applied, as when the first messages of the new run
// were missed.
func TestAcceptPeerRun(t *testing.T) {
	t0 := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	newData := func() *data {
		return &data{
			localNode:          "n1",
			hbGens:             map[string]node.Gen{"n1": {"n1": 5, "n2": 5}},
			hbPatchMsgUpdated:  map[string]time.Time{"n2": t0},
			previousRemoteInfo: map[string]remoteInfo{"n2": {}},
			peerRuns:           map[string]peerRun{},
			peerRunCandidates:  map[string]peerRunCandidate{},
			log:                plog.NewDefaultLogger(),
		}
	}
	msg := func(run string, at time.Time, gen uint64) *hbtype.Msg {
		return &hbtype.Msg{Kind: "patch", Nodename: "n2", RunID: run, UpdatedAt: at, Gen: node.Gen{"n2": gen}}
	}

	t.Run("first run seen, then later messages of it", func(t *testing.T) {
		d := newData()
		assert.True(t, d.acceptPeerRun(msg("run1", t0, 5)))
		assert.True(t, d.acceptPeerRun(msg("run1", t0.Add(time.Second), 6)))
		assert.Equal(t, uint64(5), d.hbGens["n1"]["n2"], "the applied gen is kept")
		assert.Equal(t, t0.Add(time.Second), d.peerRuns["n2"].updatedAt)
	})

	t.Run("a new run asks a full message, even with a gen above the one applied", func(t *testing.T) {
		d := newData()
		d.acceptPeerRun(msg("run1", t0, 5))
		assert.True(t, d.acceptPeerRun(msg("run2", t0.Add(5*time.Second), 10)))
		assert.Equal(t, uint64(0), d.hbGens["n1"]["n2"], "the peer is asked a full message")
		assert.NotContains(t, d.hbPatchMsgUpdated, "n2")
		assert.NotContains(t, d.previousRemoteInfo, "n2")
		assert.Equal(t, "run2", d.peerRuns["n2"].id)
	})

	t.Run("a message of the previous run delivered late is dropped", func(t *testing.T) {
		d := newData()
		d.acceptPeerRun(msg("run1", t0, 5))
		d.acceptPeerRun(msg("run2", t0.Add(5*time.Second), 1))
		d.hbGens["n1"]["n2"] = 3 // the new run is being applied
		assert.False(t, d.acceptPeerRun(msg("run1", t0.Add(time.Second), 6)))
		assert.Equal(t, uint64(3), d.hbGens["n1"]["n2"], "the new run is not reset")
		assert.Equal(t, "run2", d.peerRuns["n2"].id)
	})

	t.Run("a run stamped before the known one is not applied alone", func(t *testing.T) {
		d := newData()
		d.acceptPeerRun(msg("run1", t0, 5))
		assert.False(t, d.acceptPeerRun(msg("run0", t0.Add(-time.Hour), 40)), "a very old message of another run is dropped")
		assert.Equal(t, uint64(5), d.hbGens["n1"]["n2"], "the known run is not reset")
		assert.Equal(t, "run1", d.peerRuns["n2"].id)
	})

	t.Run("a run stamped before the known one, confirmed, is a peer whose clock was stepped back", func(t *testing.T) {
		d := newData()
		d.acceptPeerRun(msg("run1", t0, 5))
		back := t0.Add(-time.Hour)
		assert.False(t, d.acceptPeerRun(msg("run2", back, 1)))
		assert.False(t, d.acceptPeerRun(msg("run2", back, 1)), "a copy of the same message does not count")
		assert.False(t, d.acceptPeerRun(msg("run2", back.Add(time.Second), 2)))
		assert.Equal(t, uint64(5), d.hbGens["n1"]["n2"], "not switched before the confirmation")
		assert.True(t, d.acceptPeerRun(msg("run2", back.Add(2*time.Second), 3)), "the third message confirms the run")
		assert.Equal(t, uint64(0), d.hbGens["n1"]["n2"], "the peer is asked a full message")
		assert.Equal(t, "run2", d.peerRuns["n2"].id)
		assert.NotContains(t, d.peerRunCandidates, "n2")
	})

	t.Run("a run candidate not seen again is forgotten", func(t *testing.T) {
		d := newData()
		d.acceptPeerRun(msg("run1", t0, 5))
		back := t0.Add(-time.Hour)
		d.acceptPeerRun(msg("run0", back, 1))
		d.acceptPeerRun(msg("run0", back.Add(time.Second), 2))
		c := d.peerRunCandidates["n2"]
		c.seenAt = time.Now().Add(-peerRunCandidateExpire - time.Second)
		d.peerRunCandidates["n2"] = c
		assert.False(t, d.acceptPeerRun(msg("run0", back.Add(2*time.Second), 3)), "the count started over")
		assert.Equal(t, 1, d.peerRunCandidates["n2"].count)
		assert.Equal(t, "run1", d.peerRuns["n2"].id)
	})

	t.Run("a message of the known run clears the candidate", func(t *testing.T) {
		d := newData()
		d.acceptPeerRun(msg("run1", t0, 5))
		d.acceptPeerRun(msg("run0", t0.Add(-time.Hour), 1))
		assert.True(t, d.acceptPeerRun(msg("run1", t0.Add(time.Second), 6)))
		assert.NotContains(t, d.peerRunCandidates, "n2")
	})

	t.Run("a peer not telling its run is left to the gen check", func(t *testing.T) {
		d := newData()
		assert.True(t, d.acceptPeerRun(msg("", t0, 5)))
		assert.Empty(t, d.peerRuns)
		assert.Equal(t, uint64(5), d.hbGens["n1"]["n2"])
	})
}
