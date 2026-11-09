package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// The collector speaker is the first live cluster node whose collector can
// speak: a node leader whose collector lost its client, as when its
// node.uuid went, leaves the speech to the next node, rather than keeping a
// speaker-warning no other node relieves.
func TestSpeakerIsTheFirstLiveNodeAbleToSpeak(t *testing.T) {
	type peer struct {
		alive bool
		state string
	}
	for name, tc := range map[string]struct {
		peers     map[string]peer
		hasClient bool
		leader    string
		want      string
	}{
		"the leader can speak": {
			peers:     map[string]peer{"n1": {true, "speaker"}, "n3": {true, "speaker-candidate"}},
			hasClient: true, leader: "n1", want: "n1",
		},
		"the leader lost its client": {
			peers:     map[string]peer{"n1": {true, "speaker-warning"}, "n3": {true, "speaker-candidate"}},
			hasClient: true, leader: "n1", want: "n2",
		},
		"the leader and localhost lost their client": {
			peers:     map[string]peer{"n1": {true, "speaker-warning"}, "n3": {true, "speaker-candidate"}},
			hasClient: false, leader: "n1", want: "n3",
		},
		"the first node is down": {
			peers:     map[string]peer{"n1": {false, "speaker"}, "n3": {true, "speaker-candidate"}},
			hasClient: true, leader: "n2", want: "n2",
		},
		"the first node collector is disabled": {
			peers:     map[string]peer{"n1": {true, "disabled"}, "n3": {true, "speaker-candidate"}},
			hasClient: true, leader: "n1", want: "n2",
		},
		"a peer collector state not known yet": {
			peers:     map[string]peer{"n1": {true, ""}, "n3": {true, "speaker-candidate"}},
			hasClient: true, leader: "n1", want: "n2",
		},
		"a peer collector starting before localhost not speaking": {
			peers:     map[string]peer{"n1": {true, "starting"}, "n3": {true, "speaker-candidate"}},
			hasClient: true, leader: "n1", want: "n1",
		},
		"a peer collector starting after localhost": {
			peers:     map[string]peer{"n1": {true, "warning"}, "n3": {true, "starting"}},
			hasClient: true, leader: "n1", want: "n2",
		},
		"no node can speak: the leader keeps the warning": {
			peers:     map[string]peer{"n1": {true, "speaker-warning"}, "n3": {true, "warning"}},
			hasClient: false, leader: "n1", want: "n1",
		},
	} {
		t.Run(name, func(t *testing.T) {
			node.InitData()
			daemonsubsystem.InitData()
			clusternode.Set([]string{"n1", "n2", "n3"})
			t.Cleanup(func() {
				clusternode.Set(nil)
				node.InitData()
				daemonsubsystem.InitData()
			})
			for nodename, p := range tc.peers {
				if p.alive {
					node.MonitorData.Set(nodename, &node.Monitor{})
				}
				if p.state != "" {
					c := daemonsubsystem.Collector{}
					c.State = p.state
					daemonsubsystem.DataCollector.Set(nodename, &c)
				}
			}
			tr := newSpeakerT(t, &fakeRequester{})
			tr.localhost = "n2"
			tr.isSpeaker = false
			if !tc.hasClient {
				tr.client = nil
			}
			tr.nodeIsLeader = map[string]bool{tc.leader: true}

			tr.updateSpeaker()
			assert.Equal(t, tc.want, tr.speaker)
			assert.Equal(t, tc.want == "n2", tr.isSpeaker)
		})
	}
}

// With no node able to speak, the speaker is the live leader: the leader flag
// of a node gone stays set beside the one of the leader elected after it.
func TestSpeakerFallbackIsTheLiveLeader(t *testing.T) {
	node.InitData()
	daemonsubsystem.InitData()
	clusternode.Set([]string{"n1", "n2", "n3"})
	t.Cleanup(func() {
		clusternode.Set(nil)
		node.InitData()
		daemonsubsystem.InitData()
	})
	node.MonitorData.Set("n3", &node.Monitor{})
	c := daemonsubsystem.Collector{}
	c.State = "warning"
	daemonsubsystem.DataCollector.Set("n3", &c)

	tr := newSpeakerT(t, nil)
	tr.client = nil
	tr.localhost = "n2"
	tr.isSpeaker = false
	tr.nodeIsLeader = map[string]bool{"n1": true, "n3": true}
	for range 20 {
		tr.updateSpeaker()
		assert.Equal(t, "n3", tr.speaker, "n1 is gone")
	}
}

// A peer rejoining sends its node status and collector state before its
// monitor, which tells it is alive: its monitor chooses the speaker again. A
// peer rejoining speaking still, as after a split, keeps the speech, the
// first in the cluster nodes of the two speaking.
func TestARejoiningPeerSpeakingKeepsTheSpeech(t *testing.T) {
	node.InitData()
	daemonsubsystem.InitData()
	clusternode.Set([]string{"n1", "n2"})
	t.Cleanup(func() {
		clusternode.Set(nil)
		node.InitData()
		daemonsubsystem.InitData()
	})
	c := daemonsubsystem.Collector{}
	c.State = "speaker"
	daemonsubsystem.DataCollector.Set("n1", &c)

	tr := newSpeakerT(t, &fakeRequester{})
	tr.localhost = "n2"
	tr.isSpeaker = false
	tr.nodeIsLeader = map[string]bool{"n2": true}
	tr.updateSpeaker()
	require.Equal(t, "n2", tr.speaker, "n1 is not alive yet")
	require.True(t, tr.isSpeaker)

	node.MonitorData.Set("n1", &node.Monitor{})
	tr.onNodeMonitorUpdated(&msgbus.NodeMonitorUpdated{Node: "n1"})
	assert.Equal(t, "n1", tr.speaker)
	assert.False(t, tr.isSpeaker)
}

// A node becoming speaker sends the full dataset: a node not speaker dropped
// the changes it saw, and the former speaker may have failed to send them.
func TestANewSpeakerSendsTheFullDataset(t *testing.T) {
	node.InitData()
	daemonsubsystem.InitData()
	clusternode.Set([]string{"n2"})
	t.Cleanup(func() {
		clusternode.Set(nil)
		node.InitData()
		daemonsubsystem.InitData()
	})
	node.StatusData.Set("n2", &node.Status{})

	tr := newSpeakerT(t, &fakeRequester{})
	tr.localhost = "n2"
	tr.isSpeaker = false
	tr.nodeIsLeader = map[string]bool{"n2": true}
	tr.updateSpeaker()
	require.True(t, tr.isSpeaker)
	assert.Contains(t, tr.daemonStatusChange, "@n2")
}
