package collector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// setCluster sets the cluster nodes, and the peers alive with their
// collector state, "" for one not known yet.
func setCluster(t *testing.T, nodes []string, peers map[string]string) {
	node.InitData()
	daemonsubsystem.InitData()
	clusternode.Set(nodes)
	t.Cleanup(func() {
		clusternode.Set(nil)
		node.InitData()
		daemonsubsystem.InitData()
	})
	for nodename, state := range peers {
		node.MonitorData.Set(nodename, &node.Monitor{})
		setPeerState(nodename, state)
	}
}

func setPeerState(nodename, state string) {
	if state == "" {
		return
	}
	c := daemonsubsystem.Collector{}
	c.State = state
	daemonsubsystem.DataCollector.Set(nodename, &c)
}

// A peer rejoining is alive before its collector says it can speak, its
// collector starting 5s after its daemon: the node speaking keeps the speech,
// rather than leave the cluster without speaker meanwhile, and keeps it once
// the peer can speak: the speech does not move back.
func TestARejoiningPeerLeavesTheSpeechToTheSpeaker(t *testing.T) {
	setCluster(t, []string{"n1", "n2"}, nil)
	tr := newSpeakerT(t, &fakeRequester{})
	tr.localhost = "n2"
	tr.isSpeaker = false
	tr.nodeIsLeader = map[string]bool{"n2": true}
	tr.updateSpeaker()
	require.Equal(t, "n2", tr.speaker)

	node.MonitorData.Set("n1", &node.Monitor{})
	tr.onNodeMonitorUpdated(&msgbus.NodeMonitorUpdated{Node: "n1"})
	assert.Equal(t, "n2", tr.speaker, "the collector state of n1 is not known yet")

	setPeerState("n1", "starting")
	tr.onDaemonCollectorUpdated(&msgbus.DaemonCollectorUpdated{Node: "n1"})
	assert.Equal(t, "n2", tr.speaker, "the collector of n1 is starting")

	setPeerState("n1", "speaker-candidate")
	tr.onDaemonCollectorUpdated(&msgbus.DaemonCollectorUpdated{Node: "n1"})
	assert.Equal(t, "n2", tr.speaker, "n1 can speak, n2 speaks already")
	assert.True(t, tr.isSpeaker)
}

// A collector starting holds its speech until it knows the collector state
// of the live peers, its own state saying starting: restarting first in the
// cluster nodes, it does not speak beside the node speaking in its absence,
// and leaves it the speech.
func TestAStartingCollectorHoldsItsSpeech(t *testing.T) {
	setCluster(t, []string{"n1", "n2"}, map[string]string{"n2": ""})
	tr := newSpeakerT(t, &fakeRequester{})
	tr.localhost = "n1"
	tr.isSpeaker = false
	tr.nodeIsLeader = map[string]bool{"n1": true}
	tr.startingUntil = time.Now().Add(startingMax)

	tr.updateSpeaker()
	assert.True(t, tr.starting())
	assert.False(t, tr.isSpeaker, "the collector state of n2 is not known yet")
	assert.Equal(t, "starting", tr.getState())

	setPeerState("n2", "speaker")
	tr.onDaemonCollectorUpdated(&msgbus.DaemonCollectorUpdated{Node: "n2"})
	assert.False(t, tr.starting())
	assert.Equal(t, "n2", tr.speaker, "n2 speaks already")
	assert.False(t, tr.isSpeaker)
	assert.Equal(t, "speaker-candidate", tr.getState())
}

// The startup hold ends at its deadline, though the collector state of a
// live peer is still unknown. The refresh ticker chooses the speaker again
// while the collector is starting, as here.
func TestTheStartupHoldEndsAtItsDeadline(t *testing.T) {
	setCluster(t, []string{"n1", "n2"}, map[string]string{"n2": ""})
	tr := newSpeakerT(t, &fakeRequester{})
	tr.localhost = "n1"
	tr.isSpeaker = false
	tr.nodeIsLeader = map[string]bool{"n1": true}
	tr.startingUntil = time.Now().Add(-time.Second)

	tr.updateSpeaker()
	assert.False(t, tr.starting())
	assert.True(t, tr.isSpeaker)
}

// A node with no live peer has no state to wait for: it speaks at once.
func TestASingleNodeDoesNotHoldItsSpeech(t *testing.T) {
	setCluster(t, []string{"n1"}, nil)
	tr := newSpeakerT(t, &fakeRequester{})
	tr.localhost = "n1"
	tr.isSpeaker = false
	tr.nodeIsLeader = map[string]bool{"n1": true}
	tr.startingUntil = time.Now().Add(startingMax)

	tr.updateSpeaker()
	assert.False(t, tr.starting())
	assert.True(t, tr.isSpeaker)
}

// newStartingT returns the collector of nodename, starting.
func newStartingT(t *testing.T, nodename string) *T {
	tr := newSpeakerT(t, &fakeRequester{})
	tr.localhost = nodename
	tr.isSpeaker = false
	tr.nodeIsLeader = map[string]bool{"n1": true}
	tr.startingUntil = time.Now().Add(startingMax)
	return tr
}

// Two collectors starting together each see the other starting: the first in
// the cluster nodes takes the speech, the other leaves it to it, whatever the
// order they choose in. They never speak both.
func TestCollectorsStartingTogetherSpeakOnce(t *testing.T) {
	for name, order := range map[string][]string{
		"n1 chooses first": {"n1", "n2"},
		"n2 chooses first": {"n2", "n1"},
	} {
		t.Run(name, func(t *testing.T) {
			setCluster(t, []string{"n1", "n2"}, nil)
			node.MonitorData.Set("n1", &node.Monitor{})
			node.MonitorData.Set("n2", &node.Monitor{})
			trs := map[string]*T{"n1": newStartingT(t, "n1"), "n2": newStartingT(t, "n2")}
			// Each publishes its state, starting, which the other reads.
			for _, nodename := range order {
				trs[nodename].publishOnChange(trs[nodename].getState())
			}
			// The events choose the speaker again, as a collector state
			// of the other: updateSpeaker publishes the state it leads to.
			for range 3 {
				for _, nodename := range order {
					trs[nodename].updateSpeaker()
					assert.False(t, trs["n1"].isSpeaker && trs["n2"].isSpeaker, "both speak")
				}
			}
			assert.True(t, trs["n1"].isSpeaker)
			assert.False(t, trs["n2"].isSpeaker)
			assert.Equal(t, "n1", trs["n2"].speaker)
			assert.Equal(t, "speaker", daemonsubsystem.DataCollector.Get("n1").State, "published")
			assert.Equal(t, "speaker-candidate", daemonsubsystem.DataCollector.Get("n2").State, "published")
		})
	}
}

// A node speaking keeps the speech while a peer before it starts, as a peer
// rejoining: it would otherwise leave the cluster without speaker until the
// peer is done starting.
func TestASpeakerKeepsTheSpeechWhileAPeerBeforeItStarts(t *testing.T) {
	setCluster(t, []string{"n1", "n2"}, map[string]string{"n1": "starting"})
	tr := newSpeakerT(t, &fakeRequester{})
	tr.localhost = "n2"
	tr.isSpeaker = true
	tr.speaker = "n2"
	tr.nodeIsLeader = map[string]bool{"n2": true}

	tr.updateSpeaker()
	assert.Equal(t, "n2", tr.speaker)
	assert.True(t, tr.isSpeaker)
}

// A node not speaking publishes the end of its startup hold: its state goes
// from starting to speaker-candidate, though it still does not speak. The
// state stayed starting, as on the nodes after the first of a cluster all
// restarted.
func TestTheEndOfTheStartupHoldIsPublished(t *testing.T) {
	setCluster(t, []string{"n1", "n2", "n3"}, map[string]string{"n1": "", "n3": ""})
	tr := newStartingT(t, "n2")
	tr.publishOnChange(tr.getState())
	tr.updateSpeaker()
	require.Equal(t, "starting", daemonsubsystem.DataCollector.Get("n2").State, "the peer states are not known yet")

	setPeerState("n3", "starting")
	setPeerState("n1", "speaker")
	tr.onDaemonCollectorUpdated(&msgbus.DaemonCollectorUpdated{Node: "n1"})
	assert.False(t, tr.starting())
	assert.False(t, tr.isSpeaker)
	assert.Equal(t, "speaker-candidate", daemonsubsystem.DataCollector.Get("n2").State, "published")
}
