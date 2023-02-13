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

func (d *data) setSubHb() {
	d.counterCmd <- idSetSubHb
	hbModes := make([]cluster.HbMode, 0)
	nodes := make([]string, 0)
	for node := range d.subHbMode {
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
			Mode: d.subHbMode[node],
			Type: d.subHbMsgType[node],
		})
	}

	subHb := cluster.SubHb{
		Heartbeats: hbcache.Heartbeats(),
		Modes:      hbModes,
	}
	d.pending.Subsys.Hb = subHb
	// TODO Use a dedicated msg for heartbeats updates
	eventId++
	patch := make(jsondelta.Patch, 0)
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"subsys", "hb"},
		OpValue: jsondelta.NewOptValue(subHb),
		OpKind:  "replace",
	}
	patch = append(patch, op)
	if eventB, err := json.Marshal(patch); err != nil {
		d.log.Error().Err(err).Msg("setSubHb Marshal")
	} else {
		d.bus.Pub(
			msgbus.DataUpdated{RawMessage: eventB},
			labelLocalNode,
		)
	}
}

func (d *data) setMsgMode(node string, mode string) {
	d.subHbMode[node] = mode
}

// setMsgType update the sub.hb.mode.x.Type for node,
// if value is changed publish msgbus.HbMessageTypeUpdated
func (d *data) setMsgType(node string, msgType string) {
	previous := d.subHbMsgType[node]
	if msgType != previous {
		d.subHbMsgType[node] = msgType
		joinedNodes := make([]string, 0)
		for n, v := range d.subHbMsgType {
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
