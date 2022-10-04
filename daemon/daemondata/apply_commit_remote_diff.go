package daemondata

import (
	"reflect"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/xmap"
)

func (d *data) getPeersFromPrevAndPending() []string {
	nodes := make(map[string]any)
	for n := range d.pending.Cluster.Node {
		if n == d.localNode {
			continue
		}
		nodes[n] = nil
	}
	for n := range d.previousRemoteInfo {
		if n == d.localNode {
			continue
		}
		nodes[n] = nil
	}
	return xmap.Keys(nodes)
}

func (d *data) pubMsgFromNodeDataDiff() {
	for _, node := range d.getPeersFromPrevAndPending() {
		current := d.refreshPreviousUpdated(node)
		if current == nil {
			continue
		}
		d.pubMsgFromNodeStatusDiffForNode(node)
		d.pubMsgFromNodeMonitorDiffForNode(node, current)
		d.pubMsgFromNodeInstanceDiffForNode(node, current)
		d.previousRemoteInfo[node] = *current
	}
}

func (d *data) pubMsgFromNodeStatusDiffForNode(node string) {
	var (
		prevTime         remoteInfo
		nextNode         cluster.NodeData
		next, prev       cluster.NodeStatus
		hasNext, hasPrev bool
	)
	if nextNode, hasNext = d.pending.Cluster.Node[node]; hasNext {
		next = nextNode.Status
	}
	prevTime, hasPrev = d.previousRemoteInfo[node]
	prev = prevTime.nodeStatus
	onUpdate := func() {
		var changed bool
		if !reflect.DeepEqual(prev.Labels, next.Labels) {
			msgbus.PubNodeStatusLabelsUpdate(d.bus, msgbus.NodeStatusLabelsUpdated{
				Node:  node,
				Value: next.Labels.DeepCopy(),
			})
			changed = true
		}
		if changed || !reflect.DeepEqual(prev, next) {
			msgbus.PubNodeStatusUpdate(d.bus, msgbus.NodeStatusUpdated{
				Node:  node,
				Value: *next.DeepCopy(),
			})
		}
	}
	onCreate := func() {
		msgbus.PubNodeStatusLabelsUpdate(d.bus, msgbus.NodeStatusLabelsUpdated{
			Node:  node,
			Value: next.Labels.DeepCopy(),
		})
		msgbus.PubNodeStatusUpdate(d.bus, msgbus.NodeStatusUpdated{
			Node:  node,
			Value: *next.DeepCopy(),
		})
	}

	switch {
	case hasNext && hasPrev:
		onUpdate()
	case hasNext:
		onCreate()
	}
}

func (d *data) pubMsgFromNodeMonitorDiffForNode(node string, current *remoteInfo) {
	if current == nil {
		return
	}
	prevTimes, hasPrev := d.previousRemoteInfo[node]
	if !hasPrev || current.nmonUpdated.After(prevTimes.nmonUpdated) {
		localMonitor := d.pending.Cluster.Node[node].Monitor
		msgbus.PubNmonUpdated(d.bus, msgbus.NmonUpdated{
			Node:    node,
			Monitor: *localMonitor.DeepCopy(),
		})
		return
	}
}

func (d *data) refreshPreviousUpdated(node string) *remoteInfo {
	if prev, ok := d.previousRemoteInfo[node]; ok {
		if prev.gen == d.pending.Cluster.Node[node].Status.Gen[node] {
			return nil
		}
	}
	c := d.pending.Cluster.Node[node]
	result := remoteInfo{
		nodeStatus:        *c.Status.DeepCopy(),
		smonUpdated:       make(map[string]time.Time),
		instCfgUpdated:    make(map[string]time.Time),
		instStatusUpdated: make(map[string]time.Time),
	}

	nmonUpdated := c.Monitor.StatusUpdated
	if c.Monitor.GlobalExpectUpdated.After(nmonUpdated) {
		nmonUpdated = c.Monitor.GlobalExpectUpdated
	}
	result.nmonUpdated = nmonUpdated

	for p, inst := range c.Instance {
		if inst.Status != nil {
			instUpdated := inst.Status.Updated
			if inst.Status.Frozen.After(instUpdated) {
				instUpdated = inst.Status.Frozen
			}
			result.instStatusUpdated[p] = instUpdated
		}
		if inst.Config != nil {
			result.instCfgUpdated[p] = inst.Config.Updated
		}
		if inst.Monitor != nil {
			smonUpdated := inst.Monitor.StatusUpdated
			if inst.Monitor.GlobalExpectUpdated.After(smonUpdated) {
				smonUpdated = inst.Monitor.GlobalExpectUpdated
			}
			result.smonUpdated[p] = smonUpdated
		}
	}
	result.gen = c.Status.Gen[node]

	return &result
}

func getUpdatedRemoved(toPath map[string]path.T, previous, current map[string]time.Time) (updates, removes []string) {
	for s, updated := range current {
		if _, ok := toPath[s]; !ok {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			toPath[s] = p
		}
		if previousUpdated, ok := previous[s]; !ok {
			// new object
			updates = append(updates, s)
		} else if !updated.Equal(previousUpdated) {
			// update object
			updates = append(updates, s)
		}
	}
	for s := range previous {
		if _, ok := toPath[s]; !ok {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			toPath[s] = p
		}
		if _, ok := current[s]; !ok {
			removes = append(removes, s)
		}
	}
	return
}

func (d *data) pubMsgFromNodeInstanceDiffForNode(node string, current *remoteInfo) {
	var updates, removes []string
	toPath := make(map[string]path.T)
	previous, ok := d.previousRemoteInfo[node]
	if !ok {
		previous = remoteInfo{
			smonUpdated:       make(map[string]time.Time),
			instCfgUpdated:    make(map[string]time.Time),
			instStatusUpdated: make(map[string]time.Time),
		}
	}
	updates, removes = getUpdatedRemoved(toPath, previous.instCfgUpdated, current.instCfgUpdated)
	for _, s := range updates {
		msgbus.PubCfgUpdate(d.bus, s, msgbus.CfgUpdated{
			Path:   toPath[s],
			Node:   node,
			Config: *d.pending.Cluster.Node[node].Instance[s].Config.DeepCopy(),
		})
	}
	for _, s := range removes {
		msgbus.PubCfgDelete(d.bus, s, msgbus.CfgDeleted{
			Path: toPath[s],
			Node: node,
		})
	}

	updates, removes = getUpdatedRemoved(toPath, previous.instStatusUpdated, current.instStatusUpdated)
	for _, s := range updates {
		msgbus.PubInstStatusUpdated(d.bus, s, msgbus.InstStatusUpdated{
			Path:   toPath[s],
			Node:   node,
			Status: *d.pending.Cluster.Node[node].Instance[s].Status.DeepCopy(),
		})
	}
	for _, s := range removes {
		msgbus.PubInstStatusDelete(d.bus, s, msgbus.InstStatusDeleted{
			Path: toPath[s],
			Node: node,
		})
	}

	updates, removes = getUpdatedRemoved(toPath, previous.smonUpdated, current.smonUpdated)
	for _, s := range updates {
		msgbus.PubSmonUpdated(d.bus, s, msgbus.SmonUpdated{
			Path:   toPath[s],
			Node:   node,
			Status: *d.pending.Cluster.Node[node].Instance[s].Monitor.DeepCopy(),
		})
	}
	for _, s := range removes {
		msgbus.PubSmonDelete(d.bus, s, msgbus.SmonDeleted{
			Path: toPath[s],
			Node: node,
		})
	}

	for s, updated := range current.instCfgUpdated {
		var update bool
		if previousUpdated, ok := previous.instCfgUpdated[s]; !ok {
			// new cfg object
			update = true
		} else if !updated.Equal(previousUpdated) {
			// update cfg object
			update = true
		}
		if update {

		}
	}
	for s := range previous.instCfgUpdated {
		if _, ok := current.instCfgUpdated[s]; !ok {
			// removal cfg
		}
	}
}
