package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
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
			hasClient: true, leader: "n1", want: "n1",
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
