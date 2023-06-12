package cluster

import (
	"encoding/json"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/path"
)

type (
	// Data describes the full Cluster state.
	Data struct {
		Cluster Cluster `json:"cluster" yaml:"cluster"`
		Daemon  Deamon  `json:"daemon" yaml:"daemon"`
	}

	Cluster struct {
		Config Config                   `json:"config" yaml:"config"`
		Status Status                   `json:"status" yaml:"status"`
		Object map[string]object.Status `json:"object" yaml:"object"`

		Node map[string]node.Node `json:"node" yaml:"node"`
	}

	Status struct {
		IsCompat bool `json:"is_compat" yaml:"is_compat"`
		IsFrozen bool `json:"is_frozen" yaml:"is_frozen"`
	}

	Deamon struct {
		Collector DaemonCollector `json:"collector" yaml:"collector"`
		DNS       DaemonDNS       `json:"dns" yaml:"dns"`
		Hb        DaemonHb        `json:"hb" yaml:"hb"`
		Listener  DaemonListener  `json:"listener" yaml:"listener"`
		Monitor   DaemonMonitor   `json:"monitor" yaml:"monitor"`
		Nodename  string          `json:"nodename" yaml:"nodename"`
		Routines  int             `json:"routines" yaml:"routines"`
		Scheduler DaemonScheduler `json:"scheduler" yaml:"scheduler"`
	}

	DaemonHb struct {
		Streams []HeartbeatStream `json:"streams" yaml:"streams"`
		Modes   []HbMode          `json:"modes" yaml:"modes"`
	}

	HbMode struct {
		Node string `json:"node" yaml:"node"`

		// Mode is the type of hb message except when Type is patch where it is the patch queue length
		Mode string `json:"mode" yaml:"mode"`

		// Type is the hb message type (unset/ping/full/patch)
		Type string `json:"type" yaml:"type"`
	}
)

func (s *Data) DeepCopy() *Data {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	newStatus := Data{}
	if err := json.Unmarshal(b, &newStatus); err != nil {
		panic(err)
	}
	return &newStatus
}

// WithSelector purges the dataset from objects not matching the selector expression
func (s *Data) WithSelector(selector string) *Data {
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
func (s *Data) WithNamespace(namespace string) *Data {
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

// GetNodeData extracts from the cluster dataset all information relative
// to node data.
func (s *Data) GetNodeData(nodename string) *node.Node {
	if nodeData, ok := s.Cluster.Node[nodename]; ok {
		return &nodeData
	}
	return nil
}

// GetNodeStatus extracts from the cluster dataset all information relative
// to node status.
func (s *Data) GetNodeStatus(nodename string) *node.Status {
	if nodeData, ok := s.Cluster.Node[nodename]; ok {
		return &nodeData.Status
	}
	return nil
}

// GetObjectStatus extracts from the cluster dataset all information relative
// to an object.
func (s *Data) GetObjectStatus(p path.T) object.Digest {
	ps := p.String()
	data := object.NewStatus()
	data.Path = p
	data.IsCompat = s.Cluster.Status.IsCompat
	data.Object, _ = s.Cluster.Object[ps]
	for nodename, ndata := range s.Cluster.Node {
		instanceStates := instance.States{}
		instanceStates.Node.FrozenAt = ndata.Status.FrozenAt
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
		if inst.Monitor != nil {
			instanceStates.Monitor = *inst.Monitor
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
