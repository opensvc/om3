package daemondata

import (
	"context"

	"github.com/goccy/go-json"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/stringslice"
)

type (
	opSetClusterConfig struct {
		errC
		value cluster.Config
	}
)

// SetClusterConfig sets .cluster.config
func (t T) SetClusterConfig(value cluster.Config) error {
	err := make(chan error, 1)
	op := opSetClusterConfig{
		errC:  err,
		value: value,
	}
	t.cmdC <- op
	return <-err
}

func (o opSetClusterConfig) call(ctx context.Context, d *data) error {
	d.statCount[idSetClusterConfig]++
	previousNodes := d.pending.Cluster.Config.Nodes
	d.pending.Cluster.Config = o.value
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"cluster", "config"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	// TODO find more explicit method to send such events
	// Here .cluter.config is used within 'om mon' event watcher
	rootPatch := jsondelta.Patch{op}
	if eventB, err := json.Marshal(rootPatch); err != nil {
		d.log.Error().Err(err).Msg("opSetClusterConfig Marshal patch")
	} else {
		eventId++
		d.bus.Pub(msgbus.DataUpdated{RawMessage: eventB}, labelLocalNode)
	}
	d.bus.Pub(msgbus.ClusterConfigUpdated{Node: d.localNode, Value: o.value})
	removed, added := stringslice.Diff(previousNodes, o.value.Nodes)
	if len(added) > 0 {
		d.log.Debug().Msgf("added nodes: %s", added)
	}
	if len(removed) > 0 {
		d.log.Debug().Msgf("removed nodes: %s", removed)
	}
	for _, v := range added {
		d.clusterNodes[v] = struct{}{}
		d.bus.Pub(msgbus.JoinSuccess{Node: v},
			labelLocalNode,
			pubsub.Label{"added", v})
	}
	for _, v := range removed {
		delete(d.clusterNodes, v)
		d.bus.Pub(msgbus.LeaveSuccess{Node: v},
			labelLocalNode,
			pubsub.Label{"removed", v})
	}
	return nil
}
