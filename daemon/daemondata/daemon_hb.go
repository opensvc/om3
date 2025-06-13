package daemondata

import (
	"slices"
	"sort"
	"time"

	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (d *data) setDaemonHeartbeat() {
	lastMessages := make([]daemonsubsystem.HeartbeatLastMessage, 0)
	nodes := make([]string, 0)
	for node := range d.hbMsgPatchLength {
		if !slices.Contains(d.clusterData.Cluster.Config.Nodes, node) {
			// Drop not anymore in cluster config nodes
			hbcache.DropPeer(node)
			continue
		}
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	for _, node := range nodes {
		lastMessages = append(lastMessages, daemonsubsystem.HeartbeatLastMessage{
			From:        node,
			PatchLength: d.hbMsgPatchLength[node],
			Type:        d.hbMsgType[node],
		})
	}

	streams, changed := hbcache.Heartbeats()
	subHb := daemonsubsystem.Heartbeat{
		Streams:      streams,
		LastMessages: lastMessages,
		LastMessage: daemonsubsystem.HeartbeatLastMessage{
			From:        d.localNode,
			PatchLength: d.hbMsgPatchLength[d.localNode],
			Type:        d.hbMsgType[d.localNode],
		},
	}
	subHb.UpdatedAt = time.Now()
	daemonsubsystem.DataHeartbeat.Set(d.localNode, subHb.DeepCopy())
	labels := []pubsub.Label{d.labelLocalhost}
	if changed {labels = append(labels, pubsub.Label{"changed", "true"})}
	d.publisher.Pub(&msgbus.DaemonHeartbeatUpdated{Node: d.localNode, Value: *subHb.DeepCopy()}, labels...)
}

func (d *data) setHbMsgPatchLength(node string, length int) {
	d.hbMsgPatchLength[node] = length
}

// setHbMsgType update the sub.hb.mode.x.Type for node,
// if value is changed publish msgbus.HeartbeatMessageTypeUpdated
func (d *data) setHbMsgType(node string, msgType string) {
	previous := d.hbMsgType[node]
	if msgType != previous {
		d.hbMsgType[node] = msgType
		joinedNodes := make([]string, 0)
		for n, v := range d.hbMsgType {
			if v == "patch" {
				joinedNodes = append(joinedNodes, n)
			}
		}
		d.publisher.Pub(&msgbus.HeartbeatMessageTypeUpdated{
			Node:          node,
			From:          previous,
			To:            msgType,
			Nodes:         append([]string{}, d.clusterData.Cluster.Config.Nodes...),
			JoinedNodes:   joinedNodes,
			InstalledGens: d.deepCopyLocalGens(),
		}, pubsub.Label{"node", node})
	}
}
