package daemondata

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
