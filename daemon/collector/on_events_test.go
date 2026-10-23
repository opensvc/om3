package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// TestSpeakerFollowsCollectorConfig pins that a node already elected
// speaks as soon as its collector configuration enables it, and stops as
// soon as it disables it: no NodeStatusUpdated follows a configuration
// change, so waiting for one could leave the cluster without speaker.
func TestSpeakerFollowsCollectorConfig(t *testing.T) {
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"}
	instance.StatusData.Set(p, "node1", &instance.Status{})
	defer instance.StatusData.Unset(p, "node1")

	tr, _ := newTestT(t)
	tr.resInfoToSend = make(map[string]*msgbus.InstanceResourceInfoUpdated)
	tr.resInfoSent = make(map[string]resInfoSent)
	tr.nodeIsLeader = make(map[string]bool)
	tr.disable = true

	tr.onNodeStatusUpdated(&msgbus.NodeStatusUpdated{Node: "node1", Value: node.Status{IsLeader: true}})
	require.False(t, tr.isSpeaker, "a node without collector configuration does not speak")

	configured := &collector.Config{FeederUrl: "https://collector.test/feeder", Password: "secret"}
	tr.onNodeConfigUpdated(&msgbus.NodeConfigUpdated{Node: "node1", Value: node.Config{Collector: configured}})
	require.False(t, tr.disable)
	assert.True(t, tr.isSpeaker, "the elected node speaks once configured")
	assert.Contains(t, tr.resInfoToSend, "test/svc/s1@node1", "a new speaker seeds the resource info to send")

	tr.onInstanceActionPending(pendingMsg(newTestCollectorAction(), "node2"))
	require.Len(t, tr.actionToSend, 1)

	tr.onNodeConfigUpdated(&msgbus.NodeConfigUpdated{Node: "node1", Value: node.Config{}})
	require.True(t, tr.disable)
	assert.False(t, tr.isSpeaker, "the elected node stops speaking once unconfigured")
	assert.Empty(t, tr.actionToSend, "a former speaker drops the actions it queued")
}
