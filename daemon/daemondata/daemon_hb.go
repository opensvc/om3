package daemondata

import (
	"encoding/json"
	"sort"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
	"github.com/opensvc/om3/util/stringslice"
)

func (d *data) setDaemonHb() {
	d.statCount[idSetDaemonHb]++
	hbModes := make([]cluster.HbMode, 0)
	nodes := make([]string, 0)
	for node := range d.hbMsgMode {
		if !stringslice.Has(node, d.pending.Cluster.Config.Nodes) {
			// Drop not anymore in cluster config nodes
			hbcache.DropPeer(node)
			continue
		}
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	for _, node := range nodes {
		hbModes = append(hbModes, cluster.HbMode{
			Node: node,
			Mode: d.hbMsgMode[node],
			Type: d.hbMsgType[node],
		})
	}

	subHb := cluster.DaemonHb{
		Streams: hbcache.Heartbeats(),
		Modes:   hbModes,
	}
	d.pending.Daemon.Hb = subHb
	// TODO Use a dedicated msg for heartbeats updates
	eventId++
	patch := make(jsondelta.Patch, 0)
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"daemon", "hb"},
		OpValue: jsondelta.NewOptValue(subHb),
		OpKind:  "replace",
	}
	patch = append(patch, op)
	if eventB, err := json.Marshal(patch); err != nil {
		d.log.Error().Err(err).Msg("setDaemonHb Marshal")
	} else {
		d.bus.Pub(msgbus.DataUpdated{RawMessage: eventB},
			labelLocalNode,
		)
	}
}

func (d *data) setHbMsgMode(node string, mode string) {
	d.hbMsgMode[node] = mode
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
		d.bus.Pub(msgbus.HbMessageTypeUpdated{
			Node:        node,
			From:        previous,
			To:          msgType,
			Nodes:       append([]string{}, d.pending.Cluster.Config.Nodes...),
			JoinedNodes: joinedNodes,
		})
	}
}
