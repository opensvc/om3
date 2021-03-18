package object

import (
	"strings"

	"github.com/fatih/color"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/render/tree"
)

type (
	// Status is a composite extract of different parts of
	// the cluster status.
	Status struct {
		Path      Path                      `json:"path"`
		Compat    bool                      `json:"compat"`
		Object    AggregatedStatus          `json:"service"`
		Instances map[string]InstanceStates `json:"instances"`
	}

	// AggregatedStatus contains the object states obtained via
	// aggregation of all instances states.
	AggregatedStatus struct {
		Avail       status.T      `json:"avail,omitempty"`
		Overall     status.T      `json:"overall,omitempty"`
		Frozen      string        `json:"frozen,omitempty"`
		Placement   string        `json:"placement,omitempty"`
		Provisioned provisioned.T `json:"provisioned,omitempty"`
	}

	// InstanceStates groups config and status of the object as seen by the daemon.
	InstanceStates struct {
		Config InstanceConfig
		Status InstanceStatus
	}
)

// Render returns a human friendy string representation of the type instance.
func (t Status) Render() string {
	tree := t.Tree()
	return tree.Render()
}

// Tree returns a tree loaded with the type instance.
func (t Status) Tree() *tree.Tree {
	tree := tree.New()
	t.LoadTreeNode(tree.Head())
	return tree
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t Status) LoadTreeNode(head *tree.Node) {
	//colors := palette.New(config.Node.Palette)
	head.AddColumn().AddText(t.Path.String()).SetColor(color.Bold)
	head.AddColumn()
	head.AddColumn().AddText(t.Object.Avail.ColorString())
	head.AddColumn().AddText(t.DescString())
	for nodename, ndata := range t.Instances {
		ndata.Status.Nodename = nodename
		ndata.Status.Path = t.Path
		n := head.AddNode()
		ndata.Status.LoadTreeNode(n)
	}
}

func (t Status) DescString() string {
	l := make([]string, 0)
	l = append(l, t.Object.Overall.ColorString())
	return strings.Join(l, " ")
}

// NewObjectStatus allocates and return a struct to host an objet full state dataset.
func NewObjectStatus() *Status {
	t := &Status{}
	t.Instances = make(map[string]InstanceStates)
	return t
}
