package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opDelNmon struct {
		err chan<- error
	}

	opGetNmon struct {
		node  string
		value chan<- cluster.NodeMonitor
	}

	opSetNmon struct {
		err   chan<- error
		value cluster.NodeMonitor
	}
)

// DelNmon deletes Monitor.Node.<localhost>.monitor
func DelNmon(c chan<- interface{}) error {
	err := make(chan error)
	op := opDelNmon{
		err: err,
	}
	c <- op
	return <-err
}

// GetNmon returns Monitor.Node.<node>.monitor
func GetNmon(c chan<- interface{}, node string) cluster.NodeMonitor {
	value := make(chan cluster.NodeMonitor)
	op := opGetNmon{
		value: value,
		node:  node,
	}
	c <- op
	return <-value
}

// GetNmon returns Monitor.Node.<node>.monitor
func (t T) GetNmon(node string) cluster.NodeMonitor {
	return GetNmon(t.cmdC, node)
}

// SetNmon sets Monitor.Node.<localhost>.monitor
func SetNmon(c chan<- interface{}, v cluster.NodeMonitor) error {
	err := make(chan error)
	op := opSetNmon{
		err:   err,
		value: v,
	}
	c <- op
	return <-err
}

func (o opDelNmon) setError(err error) {
	o.err <- err
}

func (o opDelNmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelNmon
	if _, ok := d.pending.Cluster.Node[d.localNode]; ok {
		delete(d.pending.Cluster.Node, d.localNode)
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"monitor"},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	d.bus.Pub(
		msgbus.NodeMonitorDeleted{
			Node: d.localNode,
		},
		labelLocalNode,
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opGetNmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNmon
	s := cluster.NodeMonitor{}
	if nodeStatus, ok := d.pending.Cluster.Node[o.node]; ok {
		s = nodeStatus.Monitor
	}
	select {
	case <-ctx.Done():
	case o.value <- s:
	}
}

func (o opSetNmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNmon
	newValue := d.pending.Cluster.Node[d.localNode]
	newValue.Monitor = o.value
	d.pending.Cluster.Node[d.localNode] = newValue
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"monitor"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(
		msgbus.NodeMonitorUpdated{
			Node:    d.localNode,
			Monitor: o.value,
		},
		labelLocalNode,
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
