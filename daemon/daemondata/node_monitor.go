package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opDelNodeMonitor struct {
		err chan<- error
	}

	opGetNodeMonitor struct {
		node  string
		value chan<- cluster.NodeMonitor
	}

	opSetNodeMonitor struct {
		err   chan<- error
		value cluster.NodeMonitor
	}
)

// DelNodeMonitor deletes Monitor.Node.<localhost>.monitor
func DelNodeMonitor(c chan<- interface{}) error {
	err := make(chan error)
	op := opDelNodeMonitor{
		err: err,
	}
	c <- op
	return <-err
}

// GetNodeMonitor returns Monitor.Node.<node>.monitor
func GetNodeMonitor(c chan<- interface{}, node string) cluster.NodeMonitor {
	value := make(chan cluster.NodeMonitor)
	op := opGetNodeMonitor{
		value: value,
		node:  node,
	}
	c <- op
	return <-value
}

// GetNodeMonitor returns Monitor.Node.<node>.monitor
func (t T) GetNodeMonitor(node string) cluster.NodeMonitor {
	return GetNodeMonitor(t.cmdC, node)
}

// SetNodeMonitor sets Monitor.Node.<localhost>.monitor
func SetNodeMonitor(c chan<- interface{}, v cluster.NodeMonitor) error {
	err := make(chan error)
	op := opSetNodeMonitor{
		err:   err,
		value: v,
	}
	c <- op
	return <-err
}

func (o opDelNodeMonitor) setError(err error) {
	o.err <- err
}

func (o opDelNodeMonitor) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelNodeMonitor
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

func (o opGetNodeMonitor) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeMonitor
	s := cluster.NodeMonitor{}
	if nodeStatus, ok := d.pending.Cluster.Node[o.node]; ok {
		s = nodeStatus.Monitor
	}
	select {
	case <-ctx.Done():
	case o.value <- s:
	}
}

func (o opSetNodeMonitor) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNodeMonitor
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
