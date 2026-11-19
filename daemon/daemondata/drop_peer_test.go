package daemondata

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
)

// A peer dropped loses its node monitor before its instance data: a reader
// finding the instances of the peer gone finds its node gone too, and does
// not take them for deleted by a node still up.
func TestDropPeerNodeMonitorFirst(t *testing.T) {
	node.InitData()
	t.Cleanup(node.InitData)
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	instance.ConfigData.Set(p, "n2", &instance.Config{Path: p})
	t.Cleanup(func() { instance.ConfigData.Unset(p, "n2") })
	node.MonitorData.Set("n2", &node.Monitor{State: node.MonitorStateIdle})
	spy := &tombstonePubSpy{}
	d := &data{
		localNode: "n1",
		log:       plog.NewDefaultLogger(),
		publisher: spy,
	}
	d.dropPeerData("n2", nil, time.Now())
	monitorAt, configAt := -1, -1
	for i, m := range spy.msgs {
		switch m.(type) {
		case *msgbus.NodeMonitorDeleted:
			monitorAt = i
		case *msgbus.InstanceConfigDeleted:
			configAt = i
		}
	}
	require.NotEqual(t, -1, monitorAt)
	require.NotEqual(t, -1, configAt)
	assert.Less(t, monitorAt, configAt, "the node monitor goes first")
	assert.Nil(t, node.MonitorData.GetByNode("n2"))
	assert.Nil(t, instance.ConfigData.GetByPathAndNode(p, "n2"))
}

// The tombstones of a peer dropped are kept: a node away may hold the version
// one deleted.
func TestDropPeerKeepsItsTombstones(t *testing.T) {
	tombstone.InitData()
	t.Cleanup(tombstone.InitData)
	tombstone.Data.Set("n2", tombstone.Record{OrchestrationID: "a", Path: "root/svc/s1"}, true)
	spy := &tombstonePubSpy{}
	d := &data{
		localNode: "n1",
		log:       plog.NewDefaultLogger(),
		publisher: spy,
	}
	d.dropPeerData("n2", nil, time.Now())
	_, ok := tombstone.Data.Get("n2", "a")
	assert.True(t, ok)
	for _, m := range spy.msgs {
		assert.NotEqual(t, "*msgbus.NodeTombstoneDeleted", fmt.Sprintf("%T", m))
	}
}
