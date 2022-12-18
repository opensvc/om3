package object

import (
	"fmt"
	"strings"

	"opensvc.com/opensvc/core/colorstatus"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/placement"
	"opensvc.com/opensvc/core/priority"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/util/render/tree"
)

type (
	// Digest is a composite extract of different parts of
	// the cluster status.
	Digest struct {
		Children  map[string]Status          `json:"children,omitempty"`
		Compat    bool                       `json:"compat"`
		Instances map[string]instance.States `json:"instances"`
		Object    Status                     `json:"service"`
		Path      path.T                     `json:"path"`
		Parents   map[string]Status          `json:"parents,omitempty"`
		Slaves    map[string]Status          `json:"slaves,omitempty"`
	}

	// Status contains the object states obtained via
	// aggregation of all instances states. It exists when a instance config exists somewhere
	Status struct {
		Avail            status.T         `json:"avail"`
		FlexTarget       int              `json:"flex_target,omitempty"`
		FlexMin          int              `json:"flex_min,omitempty"`
		FlexMax          int              `json:"flex_max,omitempty"`
		Frozen           string           `json:"frozen"`
		Orchestrate      string           `json:"orchestrate"`
		Overall          status.T         `json:"overall"`
		PlacementPolicy  placement.Policy `json:"placement_policy"`
		PlacementState   placement.State  `json:"placement_state"`
		Priority         priority.T       `json:"priority"`
		Provisioned      provisioned.T    `json:"provisioned"`
		Scope            []string         `json:"scope"`
		Topology         topology.T       `json:"topology"`
		UpInstancesCount int              `json:"up_instances_count"`
	}
)

// Render returns a human friendy string representation of the type instance.
func (t Digest) Render() string {
	tree := t.Tree()
	return tree.Render()
}

// Tree returns a tree loaded with the type instance.
func (t Digest) Tree() *tree.Tree {
	tree := tree.New()
	t.LoadTreeNode(tree.Head())
	return tree
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t Digest) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.Path.String()).SetColor(rawconfig.Color.Bold)
	head.AddColumn()
	head.AddColumn().AddText(colorstatus.Sprint(t.Object.Avail, rawconfig.Colorize))
	head.AddColumn().AddText(t.descString())
	instances := head.AddNode()
	instances.AddColumn().AddText("instances")
	for _, data := range t.Instances {
		n := instances.AddNode()
		data.LoadTreeNode(n)
	}
	t.loadTreeNodeParents(head)
	t.loadTreeNodeChildren(head)
	t.loadTreeNodeSlaves(head)
}

func (t Digest) loadTreeNodeParents(head *tree.Node) {
	if len(t.Parents) == 0 {
		return
	}
	n := head.AddNode()
	n.AddColumn().AddText("parents")
	for p, data := range t.Parents {
		pNode := n.AddNode()
		pNode.AddColumn().AddText(p).SetColor(rawconfig.Color.Bold)
		pNode.AddColumn()
		pNode.AddColumn().AddText(colorstatus.Sprint(data.Avail, rawconfig.Colorize))
	}
}

func (t Digest) loadTreeNodeChildren(head *tree.Node) {
	if len(t.Children) == 0 {
		return
	}
	n := head.AddNode()
	n.AddColumn().AddText("children")
	for p, data := range t.Children {
		pNode := n.AddNode()
		pNode.AddColumn().AddText(p).SetColor(rawconfig.Color.Bold)
		pNode.AddColumn()
		pNode.AddColumn().AddText(colorstatus.Sprint(data.Avail, rawconfig.Colorize))
	}
}

func (t Digest) loadTreeNodeSlaves(head *tree.Node) {
	if len(t.Slaves) == 0 {
		return
	}
	n := head.AddNode()
	n.AddColumn().AddText("slaves")
	for p, data := range t.Slaves {
		pNode := n.AddNode()
		pNode.AddColumn().AddText(p).SetColor(rawconfig.Color.Bold)
		pNode.AddColumn()
		pNode.AddColumn().AddText(colorstatus.Sprint(data.Avail, rawconfig.Colorize))
	}
}

// descString returns a string presenting notable information at the object,
// instances-aggregated, level.
func (t Digest) descString() string {
	l := make([]string, 0)

	// Overall if warn. Else no need to repeat an info we can guess from Avail.
	if t.Object.Overall == status.Warn {
		l = append(l, colorstatus.Sprint(t.Object.Overall, rawconfig.Colorize))
	}

	// Placement
	switch t.Object.PlacementState {
	case placement.Optimal, placement.NotApplicable:
	default:
		l = append(l, rawconfig.Colorize.Warning(fmt.Sprintf("%s placement", t.Object.PlacementState)))
	}

	// Agent compatibility
	if !t.Compat {
		l = append(l, rawconfig.Colorize.Error("incompatible versions"))
	}

	return strings.Join(l, " ")
}

// NewStatus allocates and return a struct to host an objet full state dataset.
func NewStatus() *Digest {
	t := &Digest{}
	t.Instances = make(map[string]instance.States)
	t.Parents = make(map[string]Status)
	t.Children = make(map[string]Status)
	t.Slaves = make(map[string]Status)
	return t
}

func (s *Status) DeepCopy() *Status {
	return &Status{
		Avail:            s.Avail,
		Overall:          s.Overall,
		Frozen:           s.Frozen,
		Orchestrate:      s.Orchestrate,
		PlacementState:   s.PlacementState,
		PlacementPolicy:  s.PlacementPolicy,
		Provisioned:      s.Provisioned,
		Priority:         s.Priority,
		Topology:         s.Topology,
		FlexTarget:       s.FlexTarget,
		FlexMin:          s.FlexMin,
		FlexMax:          s.FlexMax,
		UpInstancesCount: s.UpInstancesCount,
		Scope:            append([]string{}, s.Scope...),
	}
}
