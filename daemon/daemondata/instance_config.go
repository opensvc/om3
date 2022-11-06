package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	opDelInstanceConfig struct {
		err  chan<- error
		path path.T
	}

	opSetInstanceConfig struct {
		err   chan<- error
		path  path.T
		value instance.Config
	}
)

// DelInstanceConfig
//
// Monitor.Node.*.services.config.*
func DelInstanceConfig(c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelInstanceConfig{
		err:  err,
		path: p,
	}
	c <- op
	return <-err
}

// SetInstanceConfig
//
// Monitor.Node.*.services.config.*
func SetInstanceConfig(dataCmdC chan<- interface{}, p path.T, v instance.Config) error {
	err := make(chan error)
	op := opSetInstanceConfig{
		err:   err,
		path:  p,
		value: v,
	}
	dataCmdC <- op
	return <-err
}

func (o opSetInstanceConfig) setError(err error) {
	select {
	case o.err <- err:
	}
}

func (o opDelInstanceConfig) setError(err error) {
	select {
	case o.err <- err:
	}
}

func (o opDelInstanceConfig) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelInstanceConfig
	s := o.path.String()
	if inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]; ok && inst.Config != nil {
		inst.Config = nil
		d.pending.Cluster.Node[d.localNode].Instance[s] = inst
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"instance", s, "config"},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	msgbus.Pub(d.bus, msgbus.CfgDeleted{
		Path: o.path,
		Node: d.localNode,
	}, pubsub.Label{"path", s})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opSetInstanceConfig) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetInstanceConfig
	var op jsondelta.Operation
	s := o.path.String()
	value := o.value.DeepCopy()
	if inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]; ok {
		inst.Config = value
		d.pending.Cluster.Node[d.localNode].Instance[s] = inst
	} else {
		d.pending.Cluster.Node[d.localNode].Instance[s] = instance.Instance{Config: value}
		op = jsondelta.Operation{
			OpPath:  jsondelta.OperationPath{"instance", s},
			OpValue: jsondelta.NewOptValue(struct{}{}),
			OpKind:  "replace",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	op = jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"instance", s, "config"},
		OpValue: jsondelta.NewOptValue(*value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)

	msgbus.Pub(d.bus, msgbus.CfgUpdated{
		Path:   o.path,
		Node:   d.localNode,
		Config: o.value,
	}, pubsub.Label{"path", s})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
