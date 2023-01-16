package daemondata

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/node"
	"opensvc.com/opensvc/core/nodesinfo"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opGetNodeStatus struct {
		node   string
		result chan<- *node.Status
	}
	opGetNodeStatusMap struct {
		result chan<- map[string]node.Status
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
func (t T) GetNodeStatus(nodename string) *node.Status {
	result := make(chan *node.Status)
	op := opGetNodeStatus{
		result: result,
		node:   nodename,
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
func (t T) GetNodeStatusMap() map[string]node.Status {
	result := make(chan map[string]node.Status)
	op := opGetNodeStatusMap{
		result: result,
	}
	t.cmdC <- op
	return <-result
}

func (o opGetNodeStatusMap) call(ctx context.Context, d *data) {
	m := make(map[string]node.Status)
	d.counterCmd <- idGetNodeStatusMap
	for nodename, nodeData := range d.pending.Cluster.Node {
		m[nodename] = *nodeData.Status.DeepCopy()
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
