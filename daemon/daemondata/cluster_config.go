package daemondata

import (
	"context"

	"github.com/goccy/go-json"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opSetClusterConfig struct {
		err   chan<- error
		value cluster.ClusterConfig
	}
)

// SetClusterConfig sets .cluster.config.
// It publishes msgbus.DataUpdated for 'om mon' clients.
// It is not pushed to heatbeats
func (t T) SetClusterConfig(value cluster.ClusterConfig) error {
	err := make(chan error)
	op := opSetClusterConfig{
		err:   err,
		value: value,
	}
	t.cmdC <- op
	return <-err
}

func (o opSetClusterConfig) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetClusterConfig
	d.pending.Cluster.Config.Nodes = o.value.Nodes
	fromRootPatch := make(jsondelta.Patch, 0)
	fromRootPatch = append(fromRootPatch, jsondelta.Operation{
		OpPath: jsondelta.OperationPath{"cluster", "config"},
		OpValue: jsondelta.NewOptValue(cluster.ClusterConfig{
			ID:    d.pending.Cluster.Config.ID,
			Name:  d.pending.Cluster.Config.Name,
			Nodes: append([]string{}, d.pending.Cluster.Config.Nodes...)}),
		OpKind: "replace",
	})
	if eventB, err := json.Marshal(fromRootPatch); err != nil {
		d.log.Error().Err(err).Msg("eventCommitPendingOps Marshal fromRootPatch")
	} else {
		eventId++
		d.bus.Pub(msgbus.DataUpdated{RawMessage: eventB}, labelLocalNode)
	}
	o.err <- nil
}
