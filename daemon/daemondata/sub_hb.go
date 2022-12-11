package daemondata

import (
	"encoding/json"
	"sort"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/hbcache"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

func (d *data) setSubHb() {
	d.counterCmd <- idSetSubHb
	hbModes := make([]cluster.HbMode, 0)
	nodes := make([]string, 0)
	for node := range d.subHbMode {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	for _, node := range nodes {
		hbModes = append(hbModes, cluster.HbMode{
			Node: node,
			Mode: d.subHbMode[node],
		})
	}

	subHb := cluster.SubHb{
		Heartbeats: hbcache.Heartbeats(),
		Modes:      hbModes,
	}
	d.pending.Sub.Hb = subHb
	// TODO Use a dedicated msg for heartbeats updates
	eventId++
	patch := make(jsondelta.Patch, 0)
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"sub", "hb"},
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
