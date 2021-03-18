package object

import (
	"fmt"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/render/palette"
	"opensvc.com/opensvc/util/render/tree"
)

// Render returns a human friendly string representation of the type instance.
func (t InstanceStatus) Render() string {
	tree := t.Tree()
	return tree.Render()
}

// Tree returns a tree loaded with the type instance.
func (t InstanceStatus) Tree() *tree.Tree {
	tree := tree.New()
	t.LoadTreeNode(tree.Head())
	return tree
}

//
// LoadTreeNode add the tree nodes representing the type instance into another
// tree, at the specified node.
//
func (t InstanceStatus) LoadTreeNode(head *tree.Node) {
	colors := palette.New(config.Node.Palette)
	head.AddColumn().AddText(t.Nodename).SetColor(colors.Bold)
	head.AddColumn()
	head.AddColumn().AddText(t.Avail.ColorString())
	head.AddColumn().AddText(fmt.Sprint(t.Priority)).SetColor(colors.Secondary)
	for rid, r := range t.Resources {
		n := head.AddNode()
		n.AddColumn().AddText(rid)
		n.AddColumn().AddText("") // flags
		n.AddColumn().AddText(r.Status.ColorString())
		n.AddColumn().AddText(r.Label)
	}
}
