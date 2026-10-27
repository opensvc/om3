package nmon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
)

// A node elected speaker publishes its status at once: the collector chooses
// its speaker from the published status, and waited for the next status
// published for another reason, as the one of a freeze, to start sending the
// daemon status and the pings.
func TestASpeakerChangeIsPublished(t *testing.T) {
	const nodename = "node1"
	publisher := &recordingPublisher{}
	m := &Manager{
		localhost: nodename,
		log:       plog.NewDefaultLogger(),
		publisher: publisher,
		livePeers: map[string]bool{nodename: true},
	}
	t.Cleanup(func() { node.StatusData.Unset(nodename) })

	// No cluster node to elect the speaker from yet, as when the
	// configuration is loaded at start.
	m.updateSpeakerAndPublish()
	assert.False(t, m.nodeStatus.IsLeader)
	assert.Empty(t, publisher.messages)

	m.clusterConfig = cluster.Config{Nodes: []string{nodename, "node2"}}
	m.updateSpeakerAndPublish()
	assert.True(t, m.nodeStatus.IsLeader)
	if assert.Len(t, publisher.messages, 1) {
		status, ok := publisher.messages[0].(*msgbus.NodeStatusUpdated)
		if assert.True(t, ok) {
			assert.True(t, status.Value.IsLeader, "the published status says speaker")
		}
	}
	assert.True(t, node.StatusData.GetByNode(nodename).IsLeader, "and so does the status data")

	m.updateSpeakerAndPublish()
	assert.Len(t, publisher.messages, 1, "an unchanged speaker is not published again")
}
