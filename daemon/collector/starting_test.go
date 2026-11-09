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
// collector starting 5s after its daemon: the node speaking keeps the speech
// until the peer says it can take it, rather than leave the cluster without
// speaker meanwhile.
func TestARejoiningPeerTakesTheSpeechOnceItsCollectorCan(t *testing.T) {
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
	assert.Equal(t, "n1", tr.speaker)
	assert.False(t, tr.isSpeaker)
}

// A collector starting holds its speech until it knows the collector state
// of the live peers, its own state saying starting: restarting first in the
// cluster nodes, it does not speak beside the node speaking in its absence.
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
	assert.Equal(t, "n1", tr.speaker)
	assert.True(t, tr.isSpeaker)
	assert.Equal(t, "speaker", tr.getState())
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
