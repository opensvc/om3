package cluster

import (
	"encoding/json"
	"strings"

	"opensvc.com/opensvc/core/instance"
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

// MarshalJSON transforms a cluster.Status struct into a []byte
//func (t *Status) MarshalJSON()([]byte, error) {}

// UnmarshalJSON loads a byte array into a cluster.Status struct
func (s *Status) UnmarshalJSON(b []byte) error {
	var (
		m   map[string]interface{}
		ds  Status
		tmp []byte
		err error
	)
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	ds.Heartbeats = make(map[string]HeartbeatThreadStatus)

	for k, v := range m {
		tmp, err = json.Marshal(v)
		switch k {
		case "cluster":
			if err := json.Unmarshal(tmp, &ds.Cluster); err != nil {
				return err
			}
		case "monitor":
			if json.Unmarshal(tmp, &ds.Monitor); err != nil {
				return err
			}
		case "scheduler":
			if json.Unmarshal(tmp, &ds.Scheduler); err != nil {
				return err
			}
		case "collector":
			if json.Unmarshal(tmp, &ds.Collector); err != nil {
				return err
			}
		case "dns":
			if json.Unmarshal(tmp, &ds.DNS); err != nil {
				return err
			}
		case "listener":
			if json.Unmarshal(tmp, &ds.Listener); err != nil {
				return err
			}
		default:
			if strings.HasPrefix(k, "hb#") {
				var hb HeartbeatThreadStatus
				if err := json.Unmarshal(tmp, &hb); err != nil {
					return err
				}
				ds.Heartbeats[k] = hb
			}
		}
	}

	*s = ds
	return nil
}

// GetNodeData extracts from the cluster dataset all information relative
// to node data.
func (s *Status) GetNodeData(nodename string) *NodeData {
	if nodeData, ok := s.Cluster.Node[nodename]; ok {
		return &nodeData
	}
	return nil
}

// GetNodeStatus extracts from the cluster dataset all information relative
// to node status.
func (s *Status) GetNodeStatus(nodename string) *NodeStatus {
	if nodeData, ok := s.Cluster.Node[nodename]; ok {
		return &nodeData.Status
	}
	return nil
}

// GetObjectStatus extracts from the cluster dataset all information relative
// to an object.
func (s *Status) GetObjectStatus(p path.T) object.Status {
	ps := p.String()
	data := object.NewStatus()
	data.Path = p
	data.Object, _ = s.Cluster.Object[ps]
	for nodename, ndata := range s.Cluster.Node {
		instanceStates := instance.States{}
		instanceStates.Node.Frozen = ndata.Status.Frozen
		instanceStates.Node.Name = nodename
		inst, ok := ndata.Instance[ps]
		if !ok {
			continue
		}
		if inst.Status != nil {
			instanceStates.Status = *inst.Status
		}
		if inst.Config != nil {
			instanceStates.Config = *inst.Config
		}
		data.Instances[nodename] = instanceStates
		for _, relative := range instanceStates.Status.Parents {
			ps := relative.String()
			data.Parents[ps] = s.Cluster.Object[ps]
		}
		for _, relative := range instanceStates.Status.Children {
			ps := relative.String()
			data.Children[ps] = s.Cluster.Object[ps]
		}
		for _, relative := range instanceStates.Status.Slaves {
			ps := relative.String()
			data.Slaves[ps] = s.Cluster.Object[ps]
		}
	}
	return *data
}
