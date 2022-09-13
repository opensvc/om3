package daemondata

import (
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
)

func (d *data) getCfgDiff() (deletes []msgbus.CfgDeleted, updates []msgbus.CfgUpdated) {
	nodes := make(map[string]struct{})
	for n := range d.pending.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range d.previous.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range nodes {
		deleted, updated := d.getCfgDiffForNode(n)
		if len(deleted) > 0 {
			deletes = append(deletes, deleted...)
		}
		if len(updated) > 0 {
			updates = append(updates, updated...)
		}
	}
	return
}

func (d *data) getStatusDiff() (deletes []msgbus.InstStatusDeleted, updates []msgbus.InstStatusUpdated) {
	nodes := make(map[string]struct{})
	for n := range d.pending.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range d.previous.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range nodes {
		deleted, updated := d.getStatusDiffForNode(n)
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
	nodes := make(map[string]struct{})
	for n := range d.pending.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range d.previous.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range nodes {
		deleted, updated := d.getSmonDiffForNode(n)
		if len(deleted) > 0 {
			deletes = append(deletes, deleted...)
		}
		if len(updated) > 0 {
			updates = append(updates, updated...)
		}
	}
	return
}

func (d *data) getCfgDiffForNode(node string) ([]msgbus.CfgDeleted, []msgbus.CfgUpdated) {
	deletes := make([]msgbus.CfgDeleted, 0)
	updates := make([]msgbus.CfgUpdated, 0)
	pendingNode, hasPendingNode := d.pending.Monitor.Nodes[node]
	previousNode, hasPreviousNode := d.previous.Monitor.Nodes[node]
	if hasPendingNode && hasPreviousNode {
		for s, pending := range pendingNode.Services.Config {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			if previous, ok := previousNode.Services.Config[s]; ok {
				// object config exists, compare date
				if !previous.Updated.Equal(pending.Updated) {
					updates = append(updates, msgbus.CfgUpdated{
						Path:   p,
						Node:   node,
						Config: *pending.DeepCopy(),
					})
				}
			} else {
				// no previous object
				updates = append(updates, msgbus.CfgUpdated{
					Path:   p,
					Node:   node,
					Config: *pending.DeepCopy(),
				})
			}
		}
		for s := range previousNode.Services.Config {
			// search for deleted objects
			if _, ok := pendingNode.Services.Config[s]; !ok {
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
		// all pending cfg are new
		for s, cfg := range pendingNode.Services.Config {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			updates = append(updates, msgbus.CfgUpdated{
				Path:   p,
				Node:   node,
				Config: *cfg.DeepCopy(),
			})
		}
	} else if hasPreviousNode {
		// all previous cfg are deleted
		for s := range previousNode.Services.Config {
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

func (d *data) getStatusDiffForNode(node string) ([]msgbus.InstStatusDeleted, []msgbus.InstStatusUpdated) {
	deletes := make([]msgbus.InstStatusDeleted, 0)
	updates := make([]msgbus.InstStatusUpdated, 0)
	pendingNode, hasPendingNode := d.pending.Monitor.Nodes[node]
	previousNode, hasPreviousNode := d.previous.Monitor.Nodes[node]
	if hasPendingNode && hasPreviousNode {
		for s, pending := range pendingNode.Services.Status {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			if previous, ok := previousNode.Services.Status[s]; ok {
				if !pending.Updated.Equal(previous.Updated) {
					updates = append(updates, msgbus.InstStatusUpdated{
						Path:   p,
						Node:   node,
						Status: *pending.DeepCopy(),
					})
				}
			} else {
				updates = append(updates, msgbus.InstStatusUpdated{
					Path:   p,
					Node:   node,
					Status: *pending.DeepCopy(),
				})
			}
		}
		for s := range previousNode.Services.Status {
			if _, ok := pendingNode.Services.Status[s]; !ok {
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
		// all pending status are new
		for s, cfg := range pendingNode.Services.Status {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			updates = append(updates, msgbus.InstStatusUpdated{
				Path:   p,
				Node:   node,
				Status: *cfg.DeepCopy(),
			})
		}
	} else if hasPreviousNode {
		// all previous status are deleted
		for s := range previousNode.Services.Status {
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
	pendingNode, hasPendingNode := d.pending.Monitor.Nodes[node]
	previousNode, hasPreviousNode := d.previous.Monitor.Nodes[node]
	if hasPendingNode && hasPreviousNode {
		for s, pending := range pendingNode.Services.Smon {
			if previous, ok := previousNode.Services.Smon[s]; ok {
				globalExpectUpdated := pending.GlobalExpectUpdated.After(previous.GlobalExpectUpdated)
				statusUpdated := pending.StatusUpdated.After(previous.StatusUpdated)
				if globalExpectUpdated || statusUpdated {
					p, err := path.Parse(s)
					if err != nil {
						continue
					}
					updates = append(updates, msgbus.SmonUpdated{
						Path:   p,
						Node:   node,
						Status: *pending.DeepCopy(),
					})
				}
			} else {
				p, err := path.Parse(s)
				if err != nil {
					continue
				}
				updates = append(updates, msgbus.SmonUpdated{
					Path:   p,
					Node:   node,
					Status: *pending.DeepCopy(),
				})
			}
		}
		for s := range previousNode.Services.Smon {
			if _, ok := pendingNode.Services.Smon[s]; !ok {
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
		// all pending status are new
		for s, cfg := range pendingNode.Services.Smon {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			updates = append(updates, msgbus.SmonUpdated{
				Path:   p,
				Node:   node,
				Status: *cfg.DeepCopy(),
			})
		}
	} else if hasPreviousNode {
		// all previous status are deleted
		for s := range previousNode.Services.Smon {
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
