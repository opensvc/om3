package daemondata

import (
	"github.com/goccy/go-json"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
)

// onClusterConfigUpdated sets .cluster.config
func (d *data) onClusterConfigUpdated(c msgbus.ClusterConfigUpdated) {
	d.statCount[idSetClusterConfig]++
	d.pending.Cluster.Config = c.Value
	for _, v := range c.NodesAdded {
		d.clusterNodes[v] = struct{}{}
	}
	for _, v := range c.NodesRemoved {
		delete(d.clusterNodes, v)
	}

	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"cluster", "config"},
		OpValue: jsondelta.NewOptValue(c.Value),
		OpKind:  "replace",
	}
	// TODO find more explicit method to send such events
	// Here .cluster.config is used within 'om mon' event watcher
	rootPatch := jsondelta.Patch{op}
	if eventB, err := json.Marshal(rootPatch); err != nil {
		d.log.Error().Err(err).Msg("opSetClusterConfig Marshal patch")
	} else {
		eventId++
		d.bus.Pub(msgbus.DataUpdated{RawMessage: eventB}, labelLocalNode)
	}
}
