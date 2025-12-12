package msgbus

import (
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// onClusterStatusUpdated updates .cluster.status
func (data *ClusterData) onClusterStatusUpdated(m *ClusterStatusUpdated) {
	data.Cluster.Status = m.Value
}

func (data *ClusterData) clusterStatusUpdated(_ pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	nodename := hostname.Hostname()
	clusterStatus := data.Cluster.Status
	l = append(l, &ClusterStatusUpdated{
		Msg: pubsub.Msg{
			Labels: pubsub.NewLabels("node", nodename, "from", "cache"),
		},
		Node:  nodename,
		Value: clusterStatus,
	})
	return l, nil
}
