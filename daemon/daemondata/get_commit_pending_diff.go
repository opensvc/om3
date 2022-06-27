package daemondata

import (
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
)

func (d *data) getCfgDiff() (deletes []moncmd.CfgDeleted, updates []moncmd.CfgUpdated) {
	nodes := make(map[string]struct{})
	for n := range d.pending.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range d.committed.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range nodes {
		cfgDeletes, cfgUpdates := d.getCfgDiffForNode(n)
		if len(cfgDeletes) > 0 {
			deletes = append(deletes, cfgDeletes...)
		}
		if len(cfgUpdates) > 0 {
			updates = append(updates, cfgUpdates...)
		}
	}
	return
}

func (d *data) getStatusDiff() (deletes []moncmd.InstStatusDeleted, updates []moncmd.InstStatusUpdated) {
	nodes := make(map[string]struct{})
	for n := range d.pending.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range d.committed.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range nodes {
		cfgDeletes, cfgUpdates := d.getStatusDiffForNode(n)
		if len(cfgDeletes) > 0 {
			deletes = append(deletes, cfgDeletes...)
		}
		if len(cfgUpdates) > 0 {
			updates = append(updates, cfgUpdates...)
		}
	}
	return
}

func (d *data) getSmonDiff() (deletes []moncmd.SmonDeleted, updates []moncmd.SmonUpdated) {
	nodes := make(map[string]struct{})
	for n := range d.pending.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range d.committed.Monitor.Nodes {
		if n == d.localNode {
			continue
		}
		nodes[n] = struct{}{}
	}
	for n := range nodes {
		deleteOnNode, updateOnNode := d.getSmonDiffForNode(n)
		if len(deleteOnNode) > 0 {
			deletes = append(deletes, deleteOnNode...)
		}
		if len(updateOnNode) > 0 {
			updates = append(updates, updateOnNode...)
		}
	}
	return
}

func (d *data) getCfgDiffForNode(node string) ([]moncmd.CfgDeleted, []moncmd.CfgUpdated) {
	deletes := make([]moncmd.CfgDeleted, 0)
	updates := make([]moncmd.CfgUpdated, 0)
	pendingNode, hasPendingNode := d.pending.Monitor.Nodes[node]
	committedNode, hasCommittedNode := d.committed.Monitor.Nodes[node]
	if hasPendingNode && hasCommittedNode {
		for s, pending := range pendingNode.Services.Config {
			if committed, ok := committedNode.Services.Config[s]; ok {
				if pending.Updated.Time().Unix() > committed.Updated.Time().Unix() {
					p, err := path.Parse(s)
					if err != nil {
						continue
					}
					updates = append(updates, moncmd.CfgUpdated{
						Path:   p,
						Node:   node,
						Config: *pending.DeepCopy(),
					})
				} else {
					for _, n := range pending.Scope {
						if n == d.localNode {
							if _, ok := d.pending.Monitor.Nodes[d.localNode].Services.Config[s]; !ok {
								// removed config file local
								p, err := path.Parse(s)
								if err != nil {
									continue
								}
								updates = append(updates, moncmd.CfgUpdated{
									Path:   p,
									Node:   node,
									Config: *pending.DeepCopy(),
								})
								break
							}
						}
					}
				}
			} else {
				p, err := path.Parse(s)
				if err != nil {
					continue
				}
				updates = append(updates, moncmd.CfgUpdated{
					Path:   p,
					Node:   node,
					Config: *pending.DeepCopy(),
				})
			}
		}
		for s := range committedNode.Services.Config {
			if _, ok := pendingNode.Services.Config[s]; !ok {
				p, err := path.Parse(s)
				if err != nil {
					continue
				}
				deletes = append(deletes, moncmd.CfgDeleted{
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
			updates = append(updates, moncmd.CfgUpdated{
				Path:   p,
				Node:   node,
				Config: *cfg.DeepCopy(),
			})
		}
	} else if hasCommittedNode {
		// all committed cfg are deleted
		for s := range committedNode.Services.Config {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			deletes = append(deletes, moncmd.CfgDeleted{
				Path: p,
				Node: node,
			})
		}
	}
	return deletes, updates
}

func (d *data) getStatusDiffForNode(node string) ([]moncmd.InstStatusDeleted, []moncmd.InstStatusUpdated) {
	deletes := make([]moncmd.InstStatusDeleted, 0)
	updates := make([]moncmd.InstStatusUpdated, 0)
	pendingNode, hasPendingNode := d.pending.Monitor.Nodes[node]
	committedNode, hasCommittedNode := d.committed.Monitor.Nodes[node]
	if hasPendingNode && hasCommittedNode {
		for s, pending := range pendingNode.Services.Status {
			if committed, ok := committedNode.Services.Status[s]; ok {
				if pending.Updated.Time().Unix() > committed.Updated.Time().Unix() {
					p, err := path.Parse(s)
					if err != nil {
						continue
					}
					updates = append(updates, moncmd.InstStatusUpdated{
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
				updates = append(updates, moncmd.InstStatusUpdated{
					Path:   p,
					Node:   node,
					Status: *pending.DeepCopy(),
				})
			}
		}
		for s := range committedNode.Services.Status {
			if _, ok := pendingNode.Services.Status[s]; !ok {
				p, err := path.Parse(s)
				if err != nil {
					continue
				}
				deletes = append(deletes, moncmd.InstStatusDeleted{
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
			updates = append(updates, moncmd.InstStatusUpdated{
				Path:   p,
				Node:   node,
				Status: *cfg.DeepCopy(),
			})
		}
	} else if hasCommittedNode {
		// all committed status are deleted
		for s := range committedNode.Services.Status {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			deletes = append(deletes, moncmd.InstStatusDeleted{
				Path: p,
				Node: node,
			})
		}
	}
	return deletes, updates
}

func (d *data) getSmonDiffForNode(node string) ([]moncmd.SmonDeleted, []moncmd.SmonUpdated) {
	deletes := make([]moncmd.SmonDeleted, 0)
	updates := make([]moncmd.SmonUpdated, 0)
	pendingNode, hasPendingNode := d.pending.Monitor.Nodes[node]
	committedNode, hasCommittedNode := d.committed.Monitor.Nodes[node]
	if hasPendingNode && hasCommittedNode {
		for s, pending := range pendingNode.Services.Smon {
			if committed, ok := committedNode.Services.Smon[s]; ok {
				globalExpectUpdated := pending.GlobalExpectUpdated.Time().Unix() > committed.GlobalExpectUpdated.Time().Unix()
				statusUpdated := pending.StatusUpdated.Time().Unix() > committed.StatusUpdated.Time().Unix()
				if globalExpectUpdated || statusUpdated {
					p, err := path.Parse(s)
					if err != nil {
						continue
					}
					updates = append(updates, moncmd.SmonUpdated{
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
				updates = append(updates, moncmd.SmonUpdated{
					Path:   p,
					Node:   node,
					Status: *pending.DeepCopy(),
				})
			}
		}
		for s := range committedNode.Services.Smon {
			if _, ok := pendingNode.Services.Smon[s]; !ok {
				p, err := path.Parse(s)
				if err != nil {
					continue
				}
				deletes = append(deletes, moncmd.SmonDeleted{
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
			updates = append(updates, moncmd.SmonUpdated{
				Path:   p,
				Node:   node,
				Status: *cfg.DeepCopy(),
			})
		}
	} else if hasCommittedNode {
		// all committed status are deleted
		for s := range committedNode.Services.Smon {
			p, err := path.Parse(s)
			if err != nil {
				continue
			}
			deletes = append(deletes, moncmd.SmonDeleted{
				Path: p,
				Node: node,
			})
		}
	}
	return deletes, updates
}
