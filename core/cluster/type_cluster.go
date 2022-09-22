package cluster

import (
	"encoding/json"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/core/path"
)

type (
	// Status describes the full Cluster state.
	Status struct {
		Cluster    Cluster                          `json:"cluster"`
		Collector  CollectorThreadStatus            `json:"collector"`
		DNS        DNSThreadStatus                  `json:"dns"`
		Scheduler  SchedulerThreadStatus            `json:"scheduler"`
		Listener   ListenerThreadStatus             `json:"listener"`
		Monitor    MonitorThreadStatus              `json:"monitor"`
		Heartbeats map[string]HeartbeatThreadStatus `json:"-"`
	}

	Cluster struct {
		Config ClusterConfig                      `json:"config"`
		Status ClusterStatus                      `json:"status"`
		Object map[string]object.AggregatedStatus `json:"object"`

		Node map[string]NodeData `json:"node"`
	}

	ClusterStatus struct {
		Compat bool `json:"compat"`
		Frozen bool `json:"frozen"`
	}

	// ClusterConfig describes the cluster id, name and nodes
	// The cluster name is used as the right most part of cluster dns
	// names.
	ClusterConfig struct {
		ID    string   `json:"id"`
		Name  string   `json:"name"`
		Nodes []string `json:"nodes"`
	}
)

func (s *Status) DeepCopy() *Status {
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	newStatus := Status{}
	if err := json.Unmarshal(b, &newStatus); err != nil {
		return nil
	}
	return &newStatus
}

// WithSelector purges the dataset from objects not matching the selector expression
func (s *Status) WithSelector(selector string) *Status {
	if selector == "" {
		return s
	}
	paths, err := objectselector.NewSelection(
		selector,
		objectselector.SelectionWithLocal(true),
	).Expand()
	if err != nil {
		return s
	}
	selected := paths.StrMap()
	for nodename, nodeData := range s.Cluster.Node {
		for ps := range nodeData.Instance {
			if !selected.Has(ps) {
				delete(s.Cluster.Node[nodename].Instance, ps)
			}
		}
	}
	for ps := range s.Cluster.Object {
		if !selected.Has(ps) {
			delete(s.Cluster.Object, ps)
		}
	}
	return s
}

// WithNamespace purges the dataset from objects not matching the namespace
func (s *Status) WithNamespace(namespace string) *Status {
	if namespace == "" {
		return s
	}
	for nodename, nodeData := range s.Cluster.Node {
		for ps := range nodeData.Instance {
			p, _ := path.Parse(ps)
			if p.Namespace != namespace {
				delete(s.Cluster.Node[nodename].Instance, ps)
			}
		}
	}
	for ps := range s.Cluster.Object {
		p, _ := path.Parse(ps)
		if p.Namespace != namespace {
			delete(s.Cluster.Object, ps)
		}
	}
	return s
}
