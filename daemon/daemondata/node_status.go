package daemondata

import (
	"context"
	"time"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/jsondelta"
)

type (
	opGetNodeStatus struct {
		errC
		node   string
		result chan<- *node.Status
	}
	opGetNodeStatusMap struct {
		errC
		result chan<- map[string]node.Status
	}
	opSetNodeStatusArbitrator struct {
		errC
		value map[string]node.ArbitratorStatus
	}
	opSetNodeStatusLabels struct {
		errC
		value nodesinfo.Labels
	}
)

// GetNodeStatus returns daemondata deep copy of cluster.Node.<node>
func (t T) GetNodeStatus(nodename string) *node.Status {
	err := make(chan error, 1)
	result := make(chan *node.Status, 1)
	op := opGetNodeStatus{
		errC:   err,
		result: result,
		node:   nodename,
	}
	t.cmdC <- op
	if <-err != nil {
		return nil
	}
	return <-result
}

func (o opGetNodeStatus) call(ctx context.Context, d *data) error {
	d.statCount[idGetNodeStatus]++
	if nodeData, ok := d.pending.Cluster.Node[o.node]; ok {
		o.result <- nodeData.Status.DeepCopy()
	} else {
		o.result <- nil
	}
	return nil
}

// GetNodeStatus returns daemondata deep copy of cluster.Node.<node>
func (t T) GetNodeStatusMap() map[string]node.Status {
	err := make(chan error, 1)
	result := make(chan map[string]node.Status, 1)
	op := opGetNodeStatusMap{
		errC:   err,
		result: result,
	}
	t.cmdC <- op
	if <-err != nil {
		return make(map[string]node.Status)
	}
	return <-result
}

func (o opGetNodeStatusMap) call(ctx context.Context, d *data) error {
	m := make(map[string]node.Status)
	d.statCount[idGetNodeStatusMap]++
	for nodename, nodeData := range d.pending.Cluster.Node {
		m[nodename] = *nodeData.Status.DeepCopy()
	}
	o.result <- m
	return nil
}

// SetNodeStatusArbitrator sets Monitor.Node.<localhost>.Status.Arbitrators
func (t T) SetNodeStatusArbitrator(a map[string]node.ArbitratorStatus) error {
	err := make(chan error, 1)
	op := opSetNodeStatusArbitrator{
		errC:  err,
		value: a,
	}
	t.cmdC <- op
	return <-err
}

func (o opSetNodeStatusArbitrator) call(ctx context.Context, d *data) error {
	d.statCount[idSetNodeStatusArbitrator]++
	v := d.pending.Cluster.Node[d.localNode]
	v.Status.Arbitrators = o.value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"status", "arbitrators"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)

	d.bus.Pub(msgbus.NodeStatusUpdated{Node: d.localNode, Value: *v.Status.DeepCopy()},
		d.labelLocalNode)
	return nil
}

// onNodeFrozenFileRemoved delete .cluster.node.<node>.status.frozen
func (d *data) onNodeFrozenFileRemoved(_ msgbus.NodeFrozenFileRemoved) {
	d.statCount[idSetNodeStatusFrozen]++
	v := d.pending.Cluster.Node[d.localNode]
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"status", "frozen"},
		OpValue: jsondelta.NewOptValue(time.Time{}),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(msgbus.Frozen{Node: hostname.Hostname(), Path: path.T{}, Value: time.Time{}},
		d.labelLocalNode)
}

// onNodeFrozenFileUpdated update .cluster.node.<node>.status.frozen
func (d *data) onNodeFrozenFileUpdated(m msgbus.NodeFrozenFileUpdated) {
	d.statCount[idSetNodeStatusFrozen]++
	v := d.pending.Cluster.Node[d.localNode]
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"status", "frozen"},
		OpValue: jsondelta.NewOptValue(m.Updated),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(msgbus.Frozen{Node: hostname.Hostname(), Path: path.T{}, Value: time.Time{}},
		d.labelLocalNode)
}

// SetNodeStatusLabels sets Monitor.Node.<localhost>.frozen
func (t T) SetNodeStatusLabels(labels nodesinfo.Labels) error {
	err := make(chan error, 1)
	op := opSetNodeStatusLabels{
		errC:  err,
		value: labels,
	}
	t.cmdC <- op
	return <-err
}

func (o opSetNodeStatusLabels) call(ctx context.Context, d *data) error {
	d.statCount[idSetNodeStatusLabels]++
	v := d.pending.Cluster.Node[d.localNode]
	v.Status.Labels = o.value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"status", "labels"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(msgbus.NodeStatusLabelsUpdated{Node: hostname.Hostname(), Value: o.value},
		d.labelLocalNode)
	d.bus.Pub(msgbus.NodeStatusUpdated{Node: d.localNode, Value: *v.Status.DeepCopy()},
		d.labelLocalNode)
	return nil
}
