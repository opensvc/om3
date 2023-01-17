package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/node"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opDelNodeMonitor struct {
		err chan<- error
	}
	opGetNodeMonitor struct {
		node  string
		value chan<- node.Monitor
	}
	opGetNodeMonitorMap struct {
		result chan<- map[string]node.Monitor
	}
	opSetNodeMonitor struct {
		err   chan<- error
		value node.Monitor
	}
)

// DelNodeMonitor deletes Monitor.Node.<localhost>.monitor
func (t T) DelNodeMonitor() error {
	err := make(chan error)
	op := opDelNodeMonitor{
		err: err,
	}
	t.cmdC <- op
	return <-err
}

// GetNodeMonitor returns Monitor.Node.<node>.monitor
func (t T) GetNodeMonitor(nodename string) node.Monitor {
	value := make(chan node.Monitor)
	op := opGetNodeMonitor{
		value: value,
		node:  nodename,
	}
	t.cmdC <- op
	return <-value
}

// GetNodeMonitorMap returns a map of NodeMonitor indexed by nodename
func (t T) GetNodeMonitorMap() map[string]node.Monitor {
	result := make(chan map[string]node.Monitor)
	op := opGetNodeMonitorMap{
		result: result,
	}
	t.cmdC <- op
	return <-result
}

// SetNodeMonitor sets Monitor.Node.<localhost>.monitor
func (t T) SetNodeMonitor(v node.Monitor) error {
	err := make(chan error)
	op := opSetNodeMonitor{
		err:   err,
		value: v,
	}
	t.cmdC <- op
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
	s := node.Monitor{}
	if nodeStatus, ok := d.pending.Cluster.Node[o.node]; ok {
		s = nodeStatus.Monitor
	}
	select {
	case <-ctx.Done():
	case o.value <- s:
	}
}

func (o opGetNodeMonitorMap) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeMonitorMap
	m := make(map[string]node.Monitor)
	for nodename, nodeData := range d.pending.Cluster.Node {
		m[nodename] = *nodeData.Monitor.DeepCopy()
	}
	o.result <- m
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
			Node:  d.localNode,
			Value: o.value,
		},
		labelLocalNode,
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
