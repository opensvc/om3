package daemondata

import (
	"slices"
	"sort"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (d *data) setDaemonHb() {
	lastMessages := make([]cluster.HbLastMessage, 0)
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
		lastMessages = append(lastMessages, cluster.HbLastMessage{
			From:        node,
			PatchLength: d.hbMsgPatchLength[node],
			Type:        d.hbMsgType[node],
		})
	}

	subHb := cluster.DaemonHb{
		Streams:      hbcache.Heartbeats(),
		LastMessages: lastMessages,
	}
	d.clusterData.Daemon.Hb = subHb
	d.bus.Pub(&msgbus.DaemonHb{Node: d.localNode, Value: subHb}, d.labelLocalNode)
}

func (d *data) setHbMsgPatchLength(node string, length int) {
	d.hbMsgPatchLength[node] = length
}

// setHbMsgType update the sub.hb.mode.x.Type for node,
// if value is changed publish msgbus.HbMessageTypeUpdated
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
		d.bus.Pub(&msgbus.HbMessageTypeUpdated{
			Node:          node,
			From:          previous,
			To:            msgType,
			Nodes:         append([]string{}, d.clusterData.Cluster.Config.Nodes...),
			JoinedNodes:   joinedNodes,
			InstalledGens: d.deepCopyLocalGens(),
		}, pubsub.Label{"node", node})
	}
}
