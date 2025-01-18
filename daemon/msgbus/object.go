package msgbus

import (
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

// OnObjectStatusDeleted delete .cluster.object.<path>
func (data *ClusterData) OnObjectStatusDeleted(m *ObjectStatusDeleted) {
	delete(data.Cluster.Object, m.Path.String())
}

// OnObjectStatusUpdated updates .cluster.object.<path>
func (data *ClusterData) OnObjectStatusUpdated(m *ObjectStatusUpdated) {
	data.Cluster.Object[m.Path.String()] = m.Value
}

// objectStatusUpdated returns []*ObjectStatusUpdated matching labels
func (data *ClusterData) objectStatusUpdated(labels pubsub.Labels) ([]any, error) {
	l := make([]any, 0)
	path := labels["path"]
	nodename := hostname.Hostname()
	if path != "" {
		if objectStatus, ok := data.Cluster.Object[path]; ok {
			p, err := naming.ParsePath(path)
			if err != nil {
				return nil, err
			}
			l = append(l, &ObjectStatusUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "path", path, "from", "cache"),
				},
				Path:  p,
				Node:  nodename,
				Value: *objectStatus.DeepCopy(),
			})
		}
	} else {
		for objectPath, objectStatus := range data.Cluster.Object {
			p, err := naming.ParsePath(objectPath)
			if err != nil {
				return nil, err
			}
			l = append(l, &ObjectStatusUpdated{
				Msg: pubsub.Msg{
					Labels: pubsub.NewLabels("node", nodename, "path", objectPath, "from", "cache"),
				},
				Path:  p,
				Node:  nodename,
				Value: *objectStatus.DeepCopy(),
			})
		}
	}
	return l, nil
}
