package object

import (
	"github.com/fatih/color"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/render/tree"
)

type (
	// ObjectStatus is a composite extract of different parts of
	// the cluster status.
	ObjectStatus struct {
		Path      Path                            `json:"path"`
		Compat    bool                            `json:"compat"`
		Object    AggregatedStatus                `json:"service"`
		Instances map[string]ObjectStatusInstance `json:"instances"`
	}

	// AggregatedStatus contains the object states obtained via
	// aggregation of all instances states.
	AggregatedStatus struct {
		Avail       status.Type      `json:"avail,omitempty"`
		Overall     status.Type      `json:"overall,omitempty"`
		Frozen      string           `json:"frozen,omitempty"`
		Placement   string           `json:"placement,omitempty"`
		Provisioned provisioned.Type `json:"provisioned,omitempty"`
	}

	// ObjectStatusInstance groups config and status of the object as seen by the daemon.
	ObjectStatusInstance struct {
		Config InstanceConfigStatus
		Status InstanceStatus
	}
)

func (t ObjectStatus) Render() string {
	tree := t.Tree()
	return tree.Render()
}

func (t ObjectStatus) Tree() *tree.Tree {
	tree := tree.New()
	t.LoadTreeNode(tree.Head())
	return tree
}

func (t ObjectStatus) LoadTreeNode(head *tree.Node) {
	//colors := palette.New(config.Node.Palette)
	head.AddColumn().AddText(t.Path.String()).SetColor(color.Bold)
	for nodename, ndata := range t.Instances {
		ndata.Status.Nodename = nodename
		ndata.Status.Path = t.Path
		n := head.AddNode()
		ndata.Status.LoadTreeNode(n)
	}
}

func NewObjectStatus() *ObjectStatus {
	t := &ObjectStatus{}
	t.Instances = make(map[string]ObjectStatusInstance)
	return t
}
