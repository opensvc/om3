package msgbus

import (
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

// onClusterConfigUpdated sets .cluster.config
func (data *ClusterData) onClusterConfigUpdated(c *ClusterConfigUpdated) {
	data.Cluster.Config = c.Value
}

func (data *ClusterData) clusterConfigUpdated(_ pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	nodename := hostname.Hostname()
	clusterConfig := data.Cluster.Config.DeepCopy()
	l = append(l, &ClusterConfigUpdated{
		Msg: pubsub.Msg{
			Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
		},
		Node:         nodename,
		Value:        *clusterConfig,
		NodesAdded:   append([]string{}, clusterConfig.Nodes...),
		NodesRemoved: []string{},
	})
	return l, nil
}
