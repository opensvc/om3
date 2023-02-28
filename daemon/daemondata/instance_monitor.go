package daemondata

import (
	"context"
	"time"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	opDelInstanceMonitor struct {
		errC
		path path.T
	}

	opSetInstanceMonitor struct {
		errC
		path  path.T
		value instance.Monitor
	}

	opGetInstanceMonitorMap struct {
		errC
		path   path.T
		result chan map[string]instance.Monitor
	}
)

// DelInstanceMonitor
//
// cluster.node.<localhost>.instance.<path>.monitor
func (t T) DelInstanceMonitor(p path.T) error {
	err := make(chan error, 1)
	op := opDelInstanceMonitor{
		errC: err,
		path: p,
	}
	t.cmdC <- op
	return <-err
}

// SetInstanceMonitor
//
// cluster.node.<localhost>.instance.<path>.monitor
func (t T) SetInstanceMonitor(p path.T, v instance.Monitor) error {
	err := make(chan error, 1)
	v.UpdatedAt = time.Now()
	op := opSetInstanceMonitor{
		errC:  err,
		path:  p,
		value: v,
	}
	t.cmdC <- op
	return <-err
}

func (o opDelInstanceMonitor) call(ctx context.Context, d *data) error {
	d.statCount[idDelInstanceMonitor]++
	s := o.path.String()
	if inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]; ok && inst.Monitor != nil {
		inst.Monitor = nil
		d.pending.Cluster.Node[d.localNode].Instance[s] = inst
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"instance", s, "monitor"},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	d.bus.Pub(
		msgbus.InstanceMonitorDeleted{
			Path: o.path,
			Node: d.localNode,
		},
		pubsub.Label{"path", s},
		labelLocalNode,
	)
	return nil
}

func (o opSetInstanceMonitor) call(ctx context.Context, d *data) error {
	d.statCount[idSetInstanceMonitor]++
	var op jsondelta.Operation
	s := o.path.String()
	value := o.value.DeepCopy()
	if inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]; ok {
		inst.Monitor = value
		d.pending.Cluster.Node[d.localNode].Instance[s] = inst

	} else {
		d.pending.Cluster.Node[d.localNode].Instance[s] = instance.Instance{Monitor: value}
		op = jsondelta.Operation{
			OpPath:  jsondelta.OperationPath{"instance", s},
			OpValue: jsondelta.NewOptValue(struct{}{}),
			OpKind:  "replace",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	op = jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"instance", s, "monitor"},
		OpValue: jsondelta.NewOptValue(*value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(
		msgbus.InstanceMonitorUpdated{
			Path:  o.path,
			Node:  d.localNode,
			Value: o.value,
		},
		pubsub.Label{"path", s},
		labelLocalNode,
	)
	return nil
}

// GetInstanceMonitorMap returns a map of InstanceMonitor indexed by nodename
func (t T) GetInstanceMonitorMap(p path.T) map[string]instance.Monitor {
	err := make(chan error, 1)
	result := make(chan map[string]instance.Monitor, 1)
	op := opGetInstanceMonitorMap{
		errC:   err,
		path:   p,
		result: result,
	}
	t.cmdC <- op
	if <-err != nil {
		return make(map[string]instance.Monitor)
	}
	return <-result
}

func (o opGetInstanceMonitorMap) call(ctx context.Context, d *data) error {
	d.statCount[idGetInstanceMonitorMap]++
	m := make(map[string]instance.Monitor)
	for nodename, nodeData := range d.pending.Cluster.Node {
		if inst, ok := nodeData.Instance[o.path.String()]; ok {
			if inst.Monitor != nil {
				m[nodename] = *inst.Monitor.DeepCopy()
			}
		}
	}
	o.result <- m
	return nil
}
