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
