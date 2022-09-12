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
	for n := range d.committed.Monitor.Nodes {
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
	for n := range d.committed.Monitor.Nodes {
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
	for n := range d.committed.Monitor.Nodes {
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
	committedNode, hasCommittedNode := d.committed.Monitor.Nodes[node]
	if hasPendingNode && hasCommittedNode {
		for s, pending := range pendingNode.Services.Config {
			if committed, ok := committedNode.Services.Config[s]; ok {
				if pending.Updated.After(committed.Updated) {
					p, err := path.Parse(s)
					if err != nil {
						continue
					}
					updates = append(updates, msgbus.CfgUpdated{
						Path:   p,
						Node:   node,
						Config: *pending.DeepCopy(),
					})
				} else {
					for _, n := range pending.Scope {
						if n == d.localNode {
							if _, ok := d.pending.Monitor.Nodes[d.localNode].Services.Config[s]; !ok {
								if remoteSmon, ok := pendingNode.Services.Smon[s]; ok {
									if remoteSmon.GlobalExpect == "purged" {
										// remote service has purge in progress
										continue
									}
								}
								// removed config file local
								p, err := path.Parse(s)
								if err != nil {
									continue
								}
								updates = append(updates, msgbus.CfgUpdated{
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
				updates = append(updates, msgbus.CfgUpdated{
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
	} else if hasCommittedNode {
		// all committed cfg are deleted
		for s := range committedNode.Services.Config {
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
	committedNode, hasCommittedNode := d.committed.Monitor.Nodes[node]
	if hasPendingNode && hasCommittedNode {
		for s, pending := range pendingNode.Services.Status {
			if committed, ok := committedNode.Services.Status[s]; ok {
				if committed.Updated.Before(pending.Updated) {
					p, err := path.Parse(s)
					if err != nil {
						continue
					}
					updates = append(updates, msgbus.InstStatusUpdated{
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
				updates = append(updates, msgbus.InstStatusUpdated{
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
	} else if hasCommittedNode {
		// all committed status are deleted
		for s := range committedNode.Services.Status {
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
	committedNode, hasCommittedNode := d.committed.Monitor.Nodes[node]
	if hasPendingNode && hasCommittedNode {
		for s, pending := range pendingNode.Services.Smon {
			if committed, ok := committedNode.Services.Smon[s]; ok {
				globalExpectUpdated := pending.GlobalExpectUpdated.After(committed.GlobalExpectUpdated)
				statusUpdated := pending.StatusUpdated.After(committed.StatusUpdated)
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
		for s := range committedNode.Services.Smon {
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
	} else if hasCommittedNode {
		// all committed status are deleted
		for s := range committedNode.Services.Smon {
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
