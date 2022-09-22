package daemondata

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opGetNodeStatus struct {
		node   string
		result chan<- *cluster.TNodeStatus
	}
	opSetNodeStatusFrozen struct {
		err   chan<- error
		value time.Time
	}
)

// GetNodeStatus returns daemondata deep copy of cluster.Node.<node>
func (t T) GetNodeStatus(node string) *cluster.TNodeStatus {
	return GetNodeStatus(t.cmdC, node)
}

// GetNodeStatus returns deep copy of cluster.Node.<node>.status
func GetNodeStatus(c chan<- any, node string) *cluster.TNodeStatus {
	result := make(chan *cluster.TNodeStatus)
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

// SetNodeFrozen sets Monitor.Node.<localhost>.frozen
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
	msgbus.PubFrozen(d.bus, hostname.Hostname(), msgbus.Frozen{
		Node:  hostname.Hostname(),
		Path:  path.T{},
		Value: o.value,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
