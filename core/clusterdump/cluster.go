package clusterdump

import (
	"encoding/json"
	"sort"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
)

type (
	// Data describes the full Cluster state.
	Data struct {
		Cluster Cluster `json:"cluster"`

		Daemon daemonsubsystem.DaemonLocal `json:"daemon"`
	}

	Cluster struct {
		Config cluster.Config           `json:"config"`
		Status Status                   `json:"status"`
		Object map[string]object.Status `json:"object"`

		Node map[string]node.Node `json:"node"`
	}

	Status struct {
		IsCompat bool `json:"is_compat"`
		IsFrozen bool `json:"is_frozen"`
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

func (s *Data) ArbitratorNames() []string {
	m := make(map[string]any)
	for _, nodeData := range s.Cluster.Node {
		for name, _ := range nodeData.Status.Arbitrators {
			m[name] = nil
		}
	}
	l := make([]string, len(m))
	i := 0
	for name := range m {
		l[i] = name
		i++
	}
	sort.Strings(l)
	return l
}

func (s *Data) ObjectPaths() naming.Paths {
	allPaths := make(naming.Paths, len(s.Cluster.Object))
	i := 0
	for p := range s.Cluster.Object {
		path, _ := naming.ParsePath(p)
		allPaths[i] = path
		i++
	}
	return allPaths
}

// WithSelector purges the dataset from objects not matching the selector expression
func (s *Data) WithSelector(selector string) *Data {
	if selector == "" {
		return s
	}
	paths, err := objectselector.New(
		selector,
		objectselector.WithPaths(s.ObjectPaths()),
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
func (s *Data) WithNamespace(namespaces ...string) *Data {
	if len(namespaces) == 0 {
		return s
	}
	allowedNamespaces := make(map[string]any)
	for _, namespace := range namespaces {
		allowedNamespaces[namespace] = nil
	}
	for nodename, nodeData := range s.Cluster.Node {
		for ps := range nodeData.Instance {
			p, _ := naming.ParsePath(ps)
			if _, ok := allowedNamespaces[p.Namespace]; !ok {
				delete(s.Cluster.Node[nodename].Instance, ps)
			}
		}
	}
	for ps := range s.Cluster.Object {
		p, _ := naming.ParsePath(ps)
		if _, ok := allowedNamespaces[p.Namespace]; !ok {
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
func (s *Data) GetObjectStatus(p naming.Path) object.Digest {
	ps := p.String()
	data := object.NewStatus()
	data.Path = p
	data.IsCompat = s.Cluster.Status.IsCompat
	data.Object, _ = s.Cluster.Object[ps]
	for nodename, ndata := range s.Cluster.Node {
		instanceStates := instance.States{}
		instanceStates.Path = p
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
		data.Instances = append(data.Instances, instanceStates)
	}
	return *data
}
