package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	opDelInstanceConfig struct {
		err  chan<- error
		path path.T
	}

	opGetInstanceConfig struct {
		config chan<- instance.Config
		path   path.T
		node   string
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
func (t T) DelInstanceConfig(p path.T) error {
	err := make(chan error)
	op := opDelInstanceConfig{
		err:  err,
		path: p,
	}
	t.cmdC <- op
	return <-err
}

// GetInstanceConfig
//
// Monitor.Node.<localhost>.services.status.*
func (t T) GetInstanceConfig(p path.T, node string) instance.Config {
	config := make(chan instance.Config)
	op := opGetInstanceConfig{
		config: config,
		path:   p,
		node:   node,
	}
	t.cmdC <- op
	return <-config
}

// SetInstanceConfig
//
// Monitor.Node.*.services.config.*
func (t T) SetInstanceConfig(p path.T, v instance.Config) error {
	err := make(chan error)
	op := opSetInstanceConfig{
		err:   err,
		path:  p,
		value: v,
	}
	t.cmdC <- op
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
	d.bus.Pub(
		msgbus.ConfigDeleted{
			Path: o.path,
			Node: d.localNode,
		},
		pubsub.Label{"path", s},
		labelLocalNode,
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opGetInstanceConfig) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetInstanceConfig
	s := instance.Config{}
	if nodeConfig, ok := d.pending.Cluster.Node[o.node]; ok {
		if inst, ok := nodeConfig.Instance[o.path.String()]; ok && inst.Config != nil {
			s = *inst.Config
		}
	}
	select {
	case <-ctx.Done():
	case o.config <- s:
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

	d.bus.Pub(
		msgbus.ConfigUpdated{
			Path:  o.path,
			Node:  d.localNode,
			Value: o.value,
		},
		pubsub.Label{"path", s},
		labelLocalNode,
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
