package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
)

type (
	opDelNodeMonitor struct {
		errC
	}
	opGetNodeMonitor struct {
		errC
		node  string
		value chan<- node.Monitor
	}
	opGetNodeMonitorMap struct {
		errC
		result chan<- map[string]node.Monitor
	}
	opSetNodeMonitor struct {
		errC
		value node.Monitor
	}
)

// DelNodeMonitor deletes Monitor.Node.<localhost>.monitor
func (t T) DelNodeMonitor() error {
	err := make(chan error, 1)
	op := opDelNodeMonitor{
		errC: err,
	}
	t.cmdC <- op
	return <-err
}

// GetNodeMonitor returns Monitor.Node.<node>.monitor
func (t T) GetNodeMonitor(nodename string) node.Monitor {
	err := make(chan error, 1)
	value := make(chan node.Monitor, 1)
	op := opGetNodeMonitor{
		errC:  err,
		value: value,
		node:  nodename,
	}
	t.cmdC <- op
	return <-value
}

// GetNodeMonitorMap returns a map of NodeMonitor indexed by nodename
func (t T) GetNodeMonitorMap() map[string]node.Monitor {
	err := make(chan error, 1)
	result := make(chan map[string]node.Monitor, 1)
	op := opGetNodeMonitorMap{
		errC:   err,
		result: result,
	}
	t.cmdC <- op
	if <-err != nil {
		return make(map[string]node.Monitor)
	}
	return <-result
}

// SetNodeMonitor sets Monitor.Node.<localhost>.monitor
func (t T) SetNodeMonitor(v node.Monitor) error {
	err := make(chan error, 1)
	op := opSetNodeMonitor{
		errC:  err,
		value: v,
	}
	t.cmdC <- op
	return <-err
}

func (o opDelNodeMonitor) call(ctx context.Context, d *data) error {
	d.statCount[idDelNodeMonitor]++
	if _, ok := d.pending.Cluster.Node[d.localNode]; ok {
		delete(d.pending.Cluster.Node, d.localNode)
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"monitor"},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	d.bus.Pub(msgbus.NodeMonitorDeleted{Node: d.localNode},
		d.labelLocalNode)
	return nil
}

func (o opGetNodeMonitor) call(ctx context.Context, d *data) error {
	d.statCount[idGetNodeMonitor]++
	s := node.Monitor{}
	if nodeStatus, ok := d.pending.Cluster.Node[o.node]; ok {
		s = nodeStatus.Monitor
	}
	o.value <- s
	return nil
}

func (o opGetNodeMonitorMap) call(ctx context.Context, d *data) error {
	d.statCount[idGetNodeMonitorMap]++
	m := make(map[string]node.Monitor)
	for nodename, nodeData := range d.pending.Cluster.Node {
		m[nodename] = *nodeData.Monitor.DeepCopy()
	}
	o.result <- m
	return nil
}

func (o opSetNodeMonitor) call(ctx context.Context, d *data) error {
	d.statCount[idSetNodeMonitor]++
	newValue := d.pending.Cluster.Node[d.localNode]
	newValue.Monitor = o.value
	d.pending.Cluster.Node[d.localNode] = newValue
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"monitor"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(msgbus.NodeMonitorUpdated{Node: d.localNode, Value: o.value},
		d.labelLocalNode)
	return nil
}
