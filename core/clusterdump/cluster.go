package clusterdump

import (
	"sort"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
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

// NewData initializes a new Data instance with an empty cluster and
// daemon specific to the provided nodename.
func NewData(fromNode string) *Data {
	return &Data{
		Cluster: Cluster{
			Object: make(map[string]object.Status),
			Node:   make(map[string]node.Node),
		},
		Daemon: daemonsubsystem.DaemonLocal{
			Nodename: fromNode,
		},
	}
}

// DeepCopy returns a copy of the cluster dataset sharing nothing with it.
//
// This used to be a json.Marshal followed by a json.Unmarshal of the whole
// dataset, which on an idle daemon was the single most expensive thing it
// did, and did it on the daemondata goroutine, where it blocked every other
// cluster data operation for its duration.
//
// A nil Object or Node map is kept nil rather than made empty: neither
// field has omitempty, so the two do not serialize alike.
func (s *Data) DeepCopy() *Data {
	if s == nil {
		return nil
	}
	c := *s
	c.Cluster.Config = *s.Cluster.Config.DeepCopy()
	if s.Cluster.Object != nil {
		c.Cluster.Object = make(map[string]object.Status, len(s.Cluster.Object))
		for path, objectData := range s.Cluster.Object {
			c.Cluster.Object[path] = *objectData.DeepCopy()
		}
	}
	if s.Cluster.Node != nil {
		c.Cluster.Node = make(map[string]node.Node, len(s.Cluster.Node))
		for nodename, nodeData := range s.Cluster.Node {
			c.Cluster.Node[nodename] = *nodeData.DeepCopy()
		}
	}
	return &c
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

// filter returns a view of the dataset restricted to the object paths keep
// accepts.
//
// The receiver is left untouched: Cluster.Object and every node Instance map
// are rebuilt, and everything the filter does not look at is shared with the
// receiver. Both must therefore be treated as read-only, which is what the
// callers do: they serialize the result.
func (s *Data) filter(keep func(ps string) bool) *Data {
	filtered := *s
	filtered.Cluster.Object = make(map[string]object.Status, len(s.Cluster.Object))
	for ps, objectData := range s.Cluster.Object {
		if keep(ps) {
			filtered.Cluster.Object[ps] = objectData
		}
	}
	filtered.Cluster.Node = make(map[string]node.Node, len(s.Cluster.Node))
	for nodename, nodeData := range s.Cluster.Node {
		instances := make(map[string]instance.Instance, len(nodeData.Instance))
		for ps, instanceData := range nodeData.Instance {
			if keep(ps) {
				instances[ps] = instanceData
			}
		}
		nodeData.Instance = instances
		filtered.Cluster.Node[nodename] = nodeData
	}
	return &filtered
}

// WithSelector returns a view of the dataset without the objects not matching
// the selector expression. The receiver is not modified, see filter.
func (s *Data) WithSelector(selector string) *Data {
	if selector == "" {
		return s
	}
	paths, err := objectselector.New(
		selector,
		objectselector.WithPaths(s.ObjectPaths()),
	).ExpandRelaxed()
	if err != nil {
		return s
	}
	selected := paths.StrMap()
	return s.filter(selected.Has)
}

// WithNamespace returns a view of the dataset without the objects not matching
// the namespaces. The receiver is not modified, see filter.
func (s *Data) WithNamespace(namespaces ...string) *Data {
	if len(namespaces) == 0 {
		return s
	}
	allowedNamespaces := make(map[string]any, len(namespaces))
	for _, namespace := range namespaces {
		allowedNamespaces[namespace] = nil
	}
	return s.filter(func(ps string) bool {
		p, _ := naming.ParsePath(ps)
		_, ok := allowedNamespaces[p.Namespace]
		return ok
	})
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
	for _, nodename := range data.Object.Scope {
		ndata, ok := s.Cluster.Node[nodename]
		if !ok {
			continue
		}
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
