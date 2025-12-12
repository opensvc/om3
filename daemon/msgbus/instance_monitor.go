package msgbus

import (
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// onInstanceMonitorDeleted delete .cluster.node.<node>.instance.<path>.monitor
func (data *ClusterData) onInstanceMonitorDeleted(m *InstanceMonitorDeleted) {
	s := m.Path.String()
	if inst, ok := data.Cluster.Node[m.Node].Instance[s]; ok && inst.Monitor != nil {
		inst.Monitor = nil
		data.Cluster.Node[m.Node].Instance[s] = inst
	}
}

// onInstanceMonitorUpdated updates .cluster.node.<node>.instance.<path>.monitor
func (data *ClusterData) onInstanceMonitorUpdated(m *InstanceMonitorUpdated) {
	s := m.Path.String()
	value := &m.Value
	if cnode, ok := data.Cluster.Node[m.Node]; ok {
		if cnode.Instance == nil {
			cnode.Instance = make(map[string]instance.Instance)
		}
		inst := cnode.Instance[s]
		inst.Monitor = value
		cnode.Instance[s] = inst
		data.Cluster.Node[m.Node] = cnode
	} else {
		data.Cluster.Node[m.Node] = node.Node{Instance: map[string]instance.Instance{s: {Monitor: value}}}
	}
}

// instanceMonitorUpdated returns []*InstanceMonitorUpdated matching labels
func (data *ClusterData) instanceMonitorUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	nodename := labels["node"]
	path := labels["path"]
	for n, nodeData := range data.Cluster.Node {
		if nodename != "" && nodename != n {
			continue
		}
		for instancePath, instanceData := range nodeData.Instance {
			if instanceData.Monitor == nil {
				continue
			}
			if path != "" && path != instancePath {
				continue
			}
			p, err := naming.ParsePath(instancePath)
			if err != nil {
				return nil, err
			}
			l = append(l, &InstanceMonitorUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", n, "path", instancePath, "source", "cache"),
				},
				Path:  p,
				Node:  n,
				Value: *instanceData.Monitor.DeepCopy(),
			})
		}
	}
	return l, nil
}
