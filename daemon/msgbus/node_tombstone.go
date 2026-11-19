package msgbus

import (
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// onNodeTombstoneDeleted deletes .cluster.node.<node>.tombstone.<id>
func (data *ClusterData) onNodeTombstoneDeleted(c *NodeTombstoneDeleted) {
	if cnode, ok := data.Cluster.Node[c.Node]; ok && cnode.Tombstone != nil {
		delete(cnode.Tombstone, c.ID)
		if len(cnode.Tombstone) == 0 {
			cnode.Tombstone = nil
		}
		data.Cluster.Node[c.Node] = cnode
	}
}

// onNodeTombstoneUpdated updates .cluster.node.<node>.tombstone.<id>
func (data *ClusterData) onNodeTombstoneUpdated(c *NodeTombstoneUpdated) {
	cnode, ok := data.Cluster.Node[c.Node]
	if !ok {
		cnode = node.Node{}
	}
	if cnode.Tombstone == nil {
		cnode.Tombstone = make(map[string]tombstone.Record)
	}
	cnode.Tombstone[c.ID] = c.Value.DeepCopy()
	data.Cluster.Node[c.Node] = cnode
}

// nodeTombstoneUpdated returns the NodeTombstoneUpdated events of the
// records the nodes publish, of the node of the labels or of every node.
func (data *ClusterData) nodeTombstoneUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	add := func(nodename string, nodeData node.Node) {
		for id, rec := range nodeData.Tombstone {
			l = append(l, &NodeTombstoneUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "path", rec.Path, "from", "cache"),
				},
				Node:  nodename,
				ID:    id,
				Path:  rec.Path,
				Value: rec.DeepCopy(),
			})
		}
	}
	if nodename := labels["node"]; nodename != "" {
		if nodeData, ok := data.Cluster.Node[nodename]; ok {
			add(nodename, nodeData)
		}
	} else {
		for nodename, nodeData := range data.Cluster.Node {
			add(nodename, nodeData)
		}
	}
	return l, nil
}
