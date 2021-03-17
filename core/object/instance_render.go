package object

import (
	"fmt"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/render/palette"
	"opensvc.com/opensvc/util/render/tree"
)

func (t InstanceStatus) Render() string {
	tree := t.Tree()
	return tree.Render()
}

func (t InstanceStatus) Tree() *tree.Tree {
	colors := palette.New(config.Node.Palette)
	tree := tree.New()
	tree.AddColumn().AddText(config.Node.Hostname).SetColor(colors.Bold)
	tree.AddColumn()
	tree.AddColumn().AddText(t.Avail.ColorString())
	tree.AddColumn().AddText(fmt.Sprint(t.Priority)).SetColor(colors.Secondary)
	for rid, r := range t.Resources {
		n := tree.AddNode()
		n.AddColumn().AddText(rid)
		n.AddColumn().AddText("") // flags
		n.AddColumn().AddText(r.Status.ColorString())
		n.AddColumn().AddText(r.Label)
	}
	return tree
}
