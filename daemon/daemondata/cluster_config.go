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
		value cluster.Config
	}
)

// SetClusterConfig sets .cluster.config
func (t T) SetClusterConfig(value cluster.Config) error {
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
	/*
		// TODO: do we need a Equal() ?
		if d.pending.Cluster.Config == o.value {
			o.err <- nil
			return
		}
	*/
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
	d.bus.Pub(
		msgbus.ClusterConfigUpdated{
			Node:  d.localNode,
			Value: o.value,
		},
	)
	o.err <- nil
}
