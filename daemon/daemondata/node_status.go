package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
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

// GetNodeStatusMap returns daemondata deep copy of cluster.Node.<node>
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

func (d *data) onNodeStatusUpdated(m msgbus.NodeStatusUpdated) {
	v := d.pending.Cluster.Node[d.localNode]
	v.Status = m.Value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"status"},
		OpValue: jsondelta.NewOptValue(m.Value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
}
