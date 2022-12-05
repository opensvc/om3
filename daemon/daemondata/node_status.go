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
	"opensvc.com/opensvc/util/pubsub"
)

type (
	opGetNodeStatus struct {
		node   string
		result chan<- *cluster.NodeStatus
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
	return GetNodeStatus(t.cmdC, node)
}

// GetNodeStatus returns deep copy of cluster.Node.<node>.Status
func GetNodeStatus(c chan<- any, node string) *cluster.NodeStatus {
	result := make(chan *cluster.NodeStatus)
	op := opGetNodeStatus{
		result: result,
		node:   node,
	}
	c <- op
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

// SetNodeFrozen sets Monitor.Node.<localhost>.Status.Frozen
func SetNodeFrozen(c chan<- interface{}, tm time.Time) error {
	err := make(chan error)
	op := opSetNodeStatusFrozen{
		err:   err,
		value: tm,
	}
	c <- op
	return <-err
}

func (o opSetNodeStatusFrozen) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNmon
	v := d.pending.Cluster.Node[d.localNode]
	v.Status.Frozen = o.value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"status", "frozen"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(msgbus.Frozen{
		Node:  hostname.Hostname(),
		Path:  path.T{},
		Value: o.value,
	}, pubsub.Label{"node", hostname.Hostname()})

	d.bus.Pub(msgbus.NodeStatusUpdated{
		Node:  hostname.Hostname(),
		Value: *v.Status.DeepCopy(),
	}, pubsub.Label{"node", hostname.Hostname()})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

// SetNodeStatusLabels sets Monitor.Node.<localhost>.frozen
func SetNodeStatusLabels(c chan<- interface{}, labels nodesinfo.Labels) error {
	err := make(chan error)
	op := opSetNodeStatusLabels{
		err:   err,
		value: labels,
	}
	c <- op
	return <-err
}

func (o opSetNodeStatusLabels) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNmon
	v := d.pending.Cluster.Node[d.localNode]
	v.Status.Labels = o.value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"status", "labels"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(msgbus.NodeStatusLabelsUpdated{
		Node:  hostname.Hostname(),
		Value: o.value,
	})
	d.bus.Pub(msgbus.NodeStatusUpdated{
		Node:  hostname.Hostname(),
		Value: *v.Status.DeepCopy(),
	}, pubsub.Label{"node", hostname.Hostname()})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
