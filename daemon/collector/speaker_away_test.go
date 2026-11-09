package collector

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/testhelper"
)

// newClusterTs returns the collectors of the nodes, every node alive, each
// with the state given, published for the others to read.
func newClusterTs(t *testing.T, states map[string]string) map[string]*T {
	nodes := []string{"n1", "n2", "n3"}
	setCluster(t, nodes, nil)
	trs := make(map[string]*T)
	for _, nodename := range nodes {
		node.MonitorData.Set(nodename, &node.Monitor{})
		tr := newSpeakerT(t, &fakeRequester{})
		tr.localhost = nodename
		tr.isSpeaker = false
		tr.nodeIsLeader = map[string]bool{"n1": true}
		trs[nodename] = tr
	}
	for nodename, state := range states {
		setPeerState(nodename, state)
		trs[nodename].status.State = state
	}
	return trs
}

// speakers returns the nodes whose collector speaks.
func speakers(trs map[string]*T) []string {
	var l []string
	for _, nodename := range []string{"n1", "n2", "n3"} {
		if trs[nodename].isSpeaker {
			l = append(l, nodename)
		}
	}
	return l
}

// A collector.url set in the cluster config enables the collectors of the
// nodes together: each holds its speech until the others say their state,
// rather than take them for disabled and speak a moment, and n1 alone speaks.
// The nodes apply the change in the order of the dev1 cluster: n1, n3, n2.
func TestCollectorsEnabledTogetherSpeakOnce(t *testing.T) {
	testhelper.Setup(t)
	trs := newClusterTs(t, map[string]string{"n1": "disabled", "n2": "disabled", "n3": "disabled"})
	for _, tr := range trs {
		tr.client = nil
		tr.disable = true
	}
	// The nodes apply the change before the state each publishes reaches
	// the others: they still see each other disabled.
	cfg := &collector.Config{FeederUrl: "https://collector.example.com", Password: "uuid"}
	for _, nodename := range []string{"n1", "n3", "n2"} {
		trs[nodename].applyConfig(cfg)
		setPeerState(nodename, "disabled")
		assert.LessOrEqual(t, len(speakers(trs)), 1, "after the enabling of %s: %v speak", nodename, speakers(trs))
	}
	// The states reach the others, which choose the speaker again.
	propagate := func() {
		for nodename, tr := range trs {
			setPeerState(nodename, tr.status.State)
		}
	}
	for range 3 {
		propagate()
		for _, nodename := range []string{"n1", "n3", "n2"} {
			trs[nodename].updateSpeaker()
			assert.LessOrEqual(t, len(speakers(trs)), 1, "%v speak", speakers(trs))
		}
	}
	propagate()
	assert.Equal(t, []string{"n1"}, speakers(trs))
	assert.Equal(t, "speaker-candidate", daemonsubsystem.DataCollector.Get("n2").State)
	assert.Equal(t, "speaker-candidate", daemonsubsystem.DataCollector.Get("n3").State)
}

// The speaker restarting keeps the speech: the peers wait for it rather than
// take the speech, and it takes the speech back at its start, from the mark it
// left at its stop. No other node speaks meanwhile.
func TestARestartingSpeakerKeepsTheSpeech(t *testing.T) {
	testhelper.Setup(t)
	trs := newClusterTs(t, map[string]string{"n2": "speaker-candidate", "n3": "speaker-candidate"})
	trs["n1"].updateSpeaker()
	for _, nodename := range []string{"n2", "n3"} {
		trs[nodename].updateSpeaker()
	}
	require.Equal(t, []string{"n1"}, speakers(trs))

	// n1 stops: its collector says stopping, and leaves its mark.
	trs["n1"].markSpeaker()
	setPeerState("n1", "stopping")
	trs["n1"].isSpeaker = false
	for _, nodename := range []string{"n2", "n3"} {
		trs[nodename].updateSpeaker()
		assert.Equal(t, "n1", trs[nodename].speaker, "%s waits for n1", nodename)
	}
	assert.Empty(t, speakers(trs))

	// n1 starts again: a new collector, holding its speech at first.
	n1 := newSpeakerT(t, &fakeRequester{})
	n1.localhost = "n1"
	n1.isSpeaker = false
	n1.nodeIsLeader = map[string]bool{"n1": true}
	n1.startHold(false)
	n1.readSpeakerMark(time.Now())
	n1.unmarkSpeaker()
	trs["n1"] = n1
	setPeerState("n1", "starting")
	for _, nodename := range []string{"n2", "n3"} {
		trs[nodename].updateSpeaker()
		assert.Equal(t, "n1", trs[nodename].speaker, "%s waits for n1 starting", nodename)
	}

	n1.updateSpeaker()
	assert.True(t, n1.isSpeaker, "n1 takes the speech back")
	for _, nodename := range []string{"n2", "n3"} {
		trs[nodename].updateSpeaker()
		assert.Equal(t, "n1", trs[nodename].speaker)
	}
	assert.Equal(t, []string{"n1"}, speakers(trs))
	assert.NoFileExists(t, speakerMarkFile(), "the mark is read once")
}

// A speaker away for longer than the speaker grace period, as a node down,
// leaves the speech to the next node able to speak.
func TestASpeakerAwayTooLongLosesTheSpeech(t *testing.T) {
	trs := newClusterTs(t, map[string]string{"n1": "speaker", "n3": "speaker-candidate"})
	n2 := trs["n2"]
	n2.updateSpeaker()
	require.Equal(t, "n1", n2.speaker)

	node.MonitorData.Unset("n1")
	n2.onNodeMonitorDeleted(&msgbus.NodeMonitorDeleted{Node: "n1"})
	assert.Equal(t, "n1", n2.speaker, "n2 waits for n1")
	assert.False(t, n2.isSpeaker)

	n2.speakerAwaySince = time.Now().Add(-collector.DefaultSpeakerGracePeriod)
	n2.updateSpeaker()
	assert.Equal(t, "n2", n2.speaker)
	assert.True(t, n2.isSpeaker)
}

// A speaker unable to speak, as one whose collector lost its client, leaves
// the speech at once: it is not away.
func TestASpeakerUnableToSpeakLosesTheSpeechAtOnce(t *testing.T) {
	trs := newClusterTs(t, map[string]string{"n1": "speaker", "n3": "speaker-candidate"})
	n2 := trs["n2"]
	n2.updateSpeaker()
	require.Equal(t, "n1", n2.speaker)

	setPeerState("n1", "speaker-warning")
	n2.updateSpeaker()
	assert.Equal(t, "n2", n2.speaker)
	assert.True(t, n2.isSpeaker)
}

// The mark of a speaker stopping is taken back within the speaker grace
// period only: past it, another node speaks.
func TestTheSpeakerMarkExpires(t *testing.T) {
	testhelper.Setup(t)
	tr := newSpeakerT(t, &fakeRequester{})
	tr.markSpeaker()
	tr.readSpeakerMark(time.Now())
	assert.True(t, tr.wasSpeaker(time.Now()))

	old := time.Now().Add(-collector.DefaultSpeakerGracePeriod - time.Second)
	require.NoError(t, os.Chtimes(speakerMarkFile(), old, old))
	tr.wasSpeakerUntil = time.Time{}
	tr.readSpeakerMark(time.Now())
	assert.False(t, tr.wasSpeaker(time.Now()))
}

// The nodes wait for a speaker away for the collector.speaker_grace_period of
// their configuration.
func TestTheSpeakerGracePeriodIsConfigured(t *testing.T) {
	trs := newClusterTs(t, map[string]string{"n1": "speaker", "n3": "speaker-candidate"})
	n2 := trs["n2"]
	n2.appliedConfig = &collector.Config{SpeakerGracePeriod: 5 * time.Second}
	n2.updateSpeaker()
	require.Equal(t, "n1", n2.speaker)

	setPeerState("n1", "stopping")
	n2.updateSpeaker()
	require.Equal(t, "n1", n2.speaker, "n2 waits for n1")

	n2.speakerAwaySince = time.Now().Add(-4 * time.Second)
	n2.updateSpeaker()
	assert.Equal(t, "n1", n2.speaker, "within the 5s grace")

	n2.speakerAwaySince = time.Now().Add(-5 * time.Second)
	n2.updateSpeaker()
	assert.Equal(t, "n2", n2.speaker, "past the 5s grace")

	n2.appliedConfig = nil
	assert.Equal(t, collector.DefaultSpeakerGracePeriod, n2.speakerGracePeriod(), "the default when not set")
}

// A collector stopping says stopping, and leaves its mark, only as a speaker
// feeding the collector: the peers wait for it. Else it says stopped, which
// the peers do not wait for, as for the node leader speaking with no client.
func TestTheStopState(t *testing.T) {
	testhelper.Setup(t)
	for name, tc := range map[string]struct {
		isSpeaker, hasClient bool
		want                 string
	}{
		"a speaker feeding the collector": {true, true, "stopping"},
		"a speaker with no client":        {true, false, "stopped"},
		"a node not speaking":             {false, true, "stopped"},
	} {
		t.Run(name, func(t *testing.T) {
			tr := newSpeakerT(t, &fakeRequester{})
			tr.isSpeaker = tc.isSpeaker
			if !tc.hasClient {
				tr.client = nil
			}
			_ = os.Remove(speakerMarkFile())
			assert.Equal(t, tc.want, tr.stopState())
			if tc.want == "stopping" {
				assert.FileExists(t, speakerMarkFile())
			} else {
				assert.NoFileExists(t, speakerMarkFile())
			}
		})
	}
}

// The peers do not wait for a speaker stopped, which did not feed the
// collector: the next node able to speak takes the speech at once.
func TestASpeakerStoppedLosesTheSpeechAtOnce(t *testing.T) {
	trs := newClusterTs(t, map[string]string{"n1": "speaker-warning", "n3": "speaker-candidate"})
	n2 := trs["n2"]
	n2.speaker = "n1"
	setPeerState("n1", "stopped")
	n2.updateSpeaker()
	assert.Equal(t, "n2", n2.speaker)
	assert.True(t, n2.isSpeaker)
}

// Two collectors restarting together each see the state the other left at its
// stop: they wait for the state of its new daemon, rather than take it for
// final and both speak, and the first in the cluster nodes takes the speech.
func TestCollectorsRestartingTogetherSpeakOnce(t *testing.T) {
	for _, stale := range []string{"stopping", "stopped"} {
		t.Run(stale, func(t *testing.T) {
			setCluster(t, []string{"n1", "n2"}, nil)
			node.MonitorData.Set("n1", &node.Monitor{})
			node.MonitorData.Set("n2", &node.Monitor{})
			trs := map[string]*T{"n1": newStartingT(t, "n1"), "n2": newStartingT(t, "n2")}
			setPeerState("n1", stale)
			setPeerState("n2", stale)
			// n2 chooses first, knowing of n1 the state it left only.
			trs["n2"].updateSpeaker()
			assert.False(t, trs["n2"].isSpeaker, "n2 took the %s state of n1 for final", stale)
			assert.True(t, trs["n2"].starting(), "n2 waits for the state of the new daemon of n1")
			for range 3 {
				for _, nodename := range []string{"n2", "n1"} {
					trs[nodename].updateSpeaker()
					assert.False(t, trs["n1"].isSpeaker && trs["n2"].isSpeaker, "both speak")
				}
			}
			assert.True(t, trs["n1"].isSpeaker)
			assert.False(t, trs["n2"].isSpeaker)
		})
	}
}

// A speaker restarting does not take the speech back from its mark after a
// peer, waiting for it less, took the speech: it follows the speaker its peers
// report, before the state of that speaker reaches it. It takes it back while
// its peers wait for it.
func TestARestartingSpeakerFollowsTheSpeakerItsPeersReport(t *testing.T) {
	testhelper.Setup(t)
	for name, tc := range map[string]struct {
		reported string
		want     string
	}{
		"a peer took the speech": {"n3", "n3"},
		"the peers wait for it":  {"n1", "n1"},
	} {
		t.Run(name, func(t *testing.T) {
			setCluster(t, []string{"n1", "n2", "n3"}, nil)
			node.MonitorData.Set("n2", &node.Monitor{})
			node.MonitorData.Set("n3", &node.Monitor{})
			c := daemonsubsystem.Collector{}
			c.State = "speaker-candidate"
			c.Speaker = tc.reported
			daemonsubsystem.DataCollector.Set("n2", &c)
			// The state of n3 does not reach n1 yet.

			n1 := newSpeakerT(t, &fakeRequester{})
			n1.localhost = "n1"
			n1.isSpeaker = false
			n1.nodeIsLeader = map[string]bool{"n1": true}
			n1.markSpeaker()
			n1.readSpeakerMark(time.Now())
			n1.updateSpeaker()
			assert.Equal(t, tc.want, n1.speaker)
			assert.Equal(t, tc.want == "n1", n1.isSpeaker)
		})
	}
}

// The collector publishes the speaker it takes, for a node restarting to
// follow it.
func TestTheSpeakerIsPublished(t *testing.T) {
	trs := newClusterTs(t, map[string]string{"n1": "speaker", "n3": "speaker-candidate"})
	trs["n2"].updateSpeaker()
	assert.Equal(t, "n1", daemonsubsystem.DataCollector.Get("n2").Speaker)
}
