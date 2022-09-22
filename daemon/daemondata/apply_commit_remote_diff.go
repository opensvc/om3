package daemondata

import (
	"reflect"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
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
	for n := range d.previous.Cluster.Node {
		if n == d.localNode {
			continue
		}
		nodes[n] = nil
	}
	return xmap.Keys(nodes)
}

func (d *data) pubMsgFromNodeDataDiff() {
	for _, node := range d.getPeersFromPrevAndPending() {
		d.pubMsgFromNodeStatusDiffForNode(node)
		d.pubMsgFromNodeMonitorDiffForNode(node)
	}
}

func (d *data) getInstCfgDiff() (deletes []msgbus.CfgDeleted, updates []msgbus.CfgUpdated) {
	for _, node := range d.getPeersFromPrevAndPending() {
		deleted, updated := d.getInstCfgDiffForNode(node)
		if len(deleted) > 0 {
			deletes = append(deletes, deleted...)
		}
		if len(updated) > 0 {
			updates = append(updates, updated...)
		}
	}
	return
}

func (d *data) getInstStatusDiff() (deletes []msgbus.InstStatusDeleted, updates []msgbus.InstStatusUpdated) {
	for _, node := range d.getPeersFromPrevAndPending() {
		deleted, updated := d.getInstStatusDiffForNode(node)
		if len(deleted) > 0 {
			deletes = append(deletes, deleted...)
		}
		if len(updated) > 0 {
			updates = append(updates, updated...)
		}
	}
	return
}

func (d *data) getSmonDiff() (deletes []msgbus.SmonDeleted, updates []msgbus.SmonUpdated) {
	for _, node := range d.getPeersFromPrevAndPending() {
		deleted, updated := d.getSmonDiffForNode(node)
		if len(deleted) > 0 {
			deletes = append(deletes, deleted...)
		}
		if len(updated) > 0 {
			updates = append(updates, updated...)
		}
	}
	return
}

func (d *data) getInstCfgDiffForNode(node string) ([]msgbus.CfgDeleted, []msgbus.CfgUpdated) {
	deletes := make([]msgbus.CfgDeleted, 0)
	updates := make([]msgbus.CfgUpdated, 0)
	pendingNode, hasPendingNode := d.pending.Cluster.Node[node]
	previousNode, hasPreviousNode := d.previous.Cluster.Node[node]
	if hasPendingNode && hasPreviousNode {
		for s, pendingInstance := range pendingNode.Instance {
			var previousValue *instance.Config
			var detectUpdate, detectDelete bool
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			pendingValue := pendingInstance.Config
			if previousInstance, ok := previousNode.Instance[s]; ok {
				previousValue = previousInstance.Config
			}
			if pendingValue != nil && previousValue != nil {
				// a previous config exists, compare
				if pendingValue.Updated.Equal(previousValue.Updated) {
					// not an update
					continue
				}
				// config updated
				detectUpdate = true
			} else if pendingValue == nil && previousValue != nil {
				// config deleted
				detectDelete = true
			} else if pendingValue != nil && previousValue == nil {
				// config added
				detectUpdate = true
			}
			if detectUpdate {
				updates = append(updates, msgbus.CfgUpdated{
					Path:   p,
					Node:   node,
					Config: *pendingValue.DeepCopy(),
				})
			} else if detectDelete {
				deletes = append(deletes, msgbus.CfgDeleted{
					Path: p,
					Node: node,
				})
			}
		}
		for s, previousInstance := range previousNode.Instance {
			// look for existing previous instance config, where no more instance exists
			if previousInstance.Config == nil {
				continue
			}
			if _, ok := pendingNode.Instance[s]; !ok {
				p, err := path.Parse(s)
				if err != nil {
					continue
				}
				deletes = append(deletes, msgbus.CfgDeleted{
					Path: p,
					Node: node,
				})
			}
		}
	} else if hasPendingNode {
		// all pending instance with config are new
		for s, pendingInstance := range pendingNode.Instance {
			if pendingInstance.Config == nil {
				continue
			}
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			updates = append(updates, msgbus.CfgUpdated{
				Path:   p,
				Node:   node,
				Config: *pendingInstance.Config.DeepCopy(),
			})
		}
	} else if hasPreviousNode {
		// all previous instance with config are deleted
		for s, previousInstance := range previousNode.Instance {
			if previousInstance.Config == nil {
				continue
			}
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			deletes = append(deletes, msgbus.CfgDeleted{
				Path: p,
				Node: node,
			})
		}
	}
	return deletes, updates
}

func (d *data) getInstStatusDiffForNode(node string) ([]msgbus.InstStatusDeleted, []msgbus.InstStatusUpdated) {
	deletes := make([]msgbus.InstStatusDeleted, 0)
	updates := make([]msgbus.InstStatusUpdated, 0)
	pendingNode, hasPendingNode := d.pending.Cluster.Node[node]
	previousNode, hasPreviousNode := d.previous.Cluster.Node[node]

	if hasPendingNode && hasPreviousNode {
		for s, pendingInstance := range pendingNode.Instance {
			var previousValue *instance.Status
			var detectUpdate, detectDelete bool
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			pendingValue := pendingInstance.Status
			if previousInstance, ok := previousNode.Instance[s]; ok {
				previousValue = previousInstance.Status
			}
			if pendingValue != nil && previousValue != nil {
				// a previous status exists, compare
				if pendingValue.Updated.Equal(previousValue.Updated) {
					// not an update
					continue
				}
				// status updated
				detectUpdate = true
			} else if pendingValue == nil && previousValue != nil {
				// status deleted
				detectDelete = true
			} else if pendingValue != nil && previousValue == nil {
				// status added
				detectUpdate = true
			}
			if detectUpdate {
				updates = append(updates, msgbus.InstStatusUpdated{
					Path:   p,
					Node:   node,
					Status: *pendingValue.DeepCopy(),
				})
			} else if detectDelete {
				deletes = append(deletes, msgbus.InstStatusDeleted{
					Path: p,
					Node: node,
				})
			}
		}
		for s, previousInstance := range previousNode.Instance {
			// look for existing previous instance status, where no more instance exists
			if previousInstance.Status == nil {
				continue
			}
			if _, ok := pendingNode.Instance[s]; !ok {
				p, err := path.Parse(s)
				if err != nil {
					continue
				}
				deletes = append(deletes, msgbus.InstStatusDeleted{
					Path: p,
					Node: node,
				})
			}
		}
	} else if hasPendingNode {
		// all pending instance with status are new
		for s, pendingInstance := range pendingNode.Instance {
			if pendingInstance.Status == nil {
				continue
			}
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			updates = append(updates, msgbus.InstStatusUpdated{
				Path:   p,
				Node:   node,
				Status: *pendingInstance.Status.DeepCopy(),
			})
		}
	} else if hasPreviousNode {
		// all previous instance with status are deleted
		for s, previousInstance := range previousNode.Instance {
			if previousInstance.Status == nil {
				continue
			}
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			deletes = append(deletes, msgbus.InstStatusDeleted{
				Path: p,
				Node: node,
			})
		}
	}
	return deletes, updates
}

func (d *data) getSmonDiffForNode(node string) ([]msgbus.SmonDeleted, []msgbus.SmonUpdated) {
	deletes := make([]msgbus.SmonDeleted, 0)
	updates := make([]msgbus.SmonUpdated, 0)

	pendingNode, hasPendingNode := d.pending.Cluster.Node[node]
	previousNode, hasPreviousNode := d.previous.Cluster.Node[node]

	if hasPendingNode && hasPreviousNode {
		for s, pendingInstance := range pendingNode.Instance {
			var previousValue *instance.Monitor
			var detectUpdate, detectDelete bool
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			pendingValue := pendingInstance.Monitor
			if previousInstance, ok := previousNode.Instance[s]; ok {
				previousValue = previousInstance.Monitor
			}
			if pendingValue != nil && previousValue != nil {
				// a previous monitor exists, compare
				globalExpectUpdated := pendingValue.GlobalExpectUpdated.After(previousValue.GlobalExpectUpdated)
				statusUpdated := pendingValue.StatusUpdated.After(previousValue.StatusUpdated)
				if !globalExpectUpdated && !statusUpdated {
					// not an update
					continue
				}
				// monitor updated
				detectUpdate = true
			} else if pendingValue == nil && previousValue != nil {
				// monitor deleted
				detectDelete = true
			} else if pendingValue != nil && previousValue == nil {
				// monitor added
				detectUpdate = true
			}
			if detectUpdate {
				updates = append(updates, msgbus.SmonUpdated{
					Path:   p,
					Node:   node,
					Status: *pendingValue.DeepCopy(),
				})
			} else if detectDelete {
				deletes = append(deletes, msgbus.SmonDeleted{
					Path: p,
					Node: node,
				})
			}
		}
		for s, previousInstance := range previousNode.Instance {
			// look for existing previous instance monitor, where no more instance exists
			if previousInstance.Status == nil {
				continue
			}
			if _, ok := pendingNode.Instance[s]; !ok {
				p, err := path.Parse(s)
				if err != nil {
					continue
				}
				deletes = append(deletes, msgbus.SmonDeleted{
					Path: p,
					Node: node,
				})
			}
		}
	} else if hasPendingNode {
		// all pending instance with monitor are new
		for s, pendingInstance := range pendingNode.Instance {
			if pendingInstance.Monitor == nil {
				continue
			}
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			updates = append(updates, msgbus.SmonUpdated{
				Path:   p,
				Node:   node,
				Status: *pendingInstance.Monitor.DeepCopy(),
			})
		}
	} else if hasPreviousNode {
		// all previous instance with monitor are deleted
		for s, previousInstance := range previousNode.Instance {
			if previousInstance.Monitor == nil {
				continue
			}
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			deletes = append(deletes, msgbus.SmonDeleted{
				Path: p,
				Node: node,
			})
		}
	}
	return deletes, updates
}

func (d *data) pubMsgFromNodeStatusDiffForNode(node string) {
	var (
		nextNode, prevNode cluster.NodeData
		next, prev         cluster.NodeStatus
		hasNext, hasPrev   bool
	)
	if nextNode, hasNext = d.pending.Cluster.Node[node]; hasNext {
		next = nextNode.Status
	}
	if prevNode, hasPrev = d.previous.Cluster.Node[node]; hasPrev {
		prev = prevNode.Status
	}

	onUpdate := func() {
		var changed bool
		if !reflect.DeepEqual(prev.Labels, next.Labels) {
			msgbus.PubNodeStatusLabelsUpdate(d.bus, node, msgbus.NodeStatusLabelsUpdated{
				Node: node,
				Data: next.Labels.DeepCopy(),
			})
			changed = true
		}
		if changed || !reflect.DeepEqual(prev, next) {
			msgbus.PubNodeStatusUpdate(d.bus, node, msgbus.NodeStatusUpdated{
				Node: node,
				Data: *next.DeepCopy(),
			})
		}
	}
	onCreate := func() {
		msgbus.PubNodeStatusLabelsUpdate(d.bus, node, msgbus.NodeStatusLabelsUpdated{
			Node: node,
			Data: next.Labels.DeepCopy(),
		})
		msgbus.PubNodeStatusUpdate(d.bus, node, msgbus.NodeStatusUpdated{
			Node: node,
			Data: *next.DeepCopy(),
		})
	}

	switch {
	case hasNext && hasPrev:
		onUpdate()
	case hasNext:
		onCreate()
	}
}

func (d *data) pubMsgFromNodeMonitorDiffForNode(node string) {
	var (
		nextNode, prevNode cluster.NodeData
		next, prev         cluster.NodeMonitor
		hasNext, hasPrev   bool
	)
	if nextNode, hasNext = d.pending.Cluster.Node[node]; hasNext {
		next = nextNode.Monitor
	}
	if prevNode, hasPrev = d.previous.Cluster.Node[node]; hasPrev {
		prev = prevNode.Monitor
	}

	onUpdate := func() {
		if !reflect.DeepEqual(prev, next) {
			msgbus.PubNmonUpdated(d.bus, msgbus.NmonUpdated{
				Node:    node,
				Monitor: *next.DeepCopy(),
			})
		}
	}
	onCreate := func() {
		msgbus.PubNmonUpdated(d.bus, msgbus.NmonUpdated{
			Node:    node,
			Monitor: *next.DeepCopy(),
		})
	}

	switch {
	case hasNext && hasPrev:
		onUpdate()
	case hasNext:
		onCreate()
	}
}
