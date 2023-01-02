package daemondata

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/nodesinfo"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opGetNodeStatus struct {
		node   string
		result chan<- *cluster.NodeStatus
	}
	opGetNodeStatusMap struct {
		result chan<- map[string]cluster.NodeStatus
	}
	opSetNodeStatusFrozen struct {
		err   chan<- error
		value time.Time
	}
	opSetNodeStatusLabels struct {
		err   chan<- error
		value nodesinfo.Labels
	}
)

// GetNodeStatus returns daemondata deep copy of cluster.Node.<node>
func (t T) GetNodeStatus(node string) *cluster.NodeStatus {
	result := make(chan *cluster.NodeStatus)
	op := opGetNodeStatus{
		result: result,
		node:   node,
	}
	t.cmdC <- op
	return <-result
}

func (o opGetNodeStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeStatus
	if nodeData, ok := d.pending.Cluster.Node[o.node]; ok {
		o.result <- nodeData.Status.DeepCopy()
	} else {
		o.result <- nil
	}
}

// GetNodeStatus returns daemondata deep copy of cluster.Node.<node>
func (t T) GetNodeStatusMap() map[string]cluster.NodeStatus {
	result := make(chan map[string]cluster.NodeStatus)
	op := opGetNodeStatusMap{
		result: result,
	}
	t.cmdC <- op
	return <-result
}

func (o opGetNodeStatusMap) call(ctx context.Context, d *data) {
	m := make(map[string]cluster.NodeStatus)
	d.counterCmd <- idGetNodeStatusMap
	for node, nodeData := range d.pending.Cluster.Node {
		m[node] = *nodeData.Status.DeepCopy()
	}
	o.result <- m
}

// SetNodeFrozen sets Monitor.Node.<localhost>.Status.Frozen
func (t T) SetNodeFrozen(tm time.Time) error {
	err := make(chan error)
	op := opSetNodeStatusFrozen{
		err:   err,
		value: tm,
	}
	t.cmdC <- op
	return <-err
}

func (o opSetNodeStatusFrozen) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNodeMonitor
	v := d.pending.Cluster.Node[d.localNode]
	v.Status.Frozen = o.value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"status", "frozen"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(
		msgbus.Frozen{
			Node:  hostname.Hostname(),
			Path:  path.T{},
			Value: o.value,
		},
		labelLocalNode,
	)

	d.bus.Pub(
		msgbus.NodeStatusUpdated{
			Node:  d.localNode,
			Value: *v.Status.DeepCopy(),
		},
		labelLocalNode,
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

// SetNodeStatusLabels sets Monitor.Node.<localhost>.frozen
func (t T) SetNodeStatusLabels(labels nodesinfo.Labels) error {
	err := make(chan error)
	op := opSetNodeStatusLabels{
		err:   err,
		value: labels,
	}
	t.cmdC <- op
	return <-err
}

func (o opSetNodeStatusLabels) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNodeMonitor
	v := d.pending.Cluster.Node[d.localNode]
	v.Status.Labels = o.value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"status", "labels"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(
		msgbus.NodeStatusLabelsUpdated{
			Node:  hostname.Hostname(),
			Value: o.value,
		},
		labelLocalNode,
	)
	d.bus.Pub(
		msgbus.NodeStatusUpdated{
			Node:  d.localNode,
			Value: *v.Status.DeepCopy(),
		},
		labelLocalNode,
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
