package object

import (
	"fmt"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/render/palette"
	"opensvc.com/opensvc/util/render/tree"
)

func (t InstanceStatus) Render() string {
	colors := palette.New(config.Node.Palette)
	tree := tree.New()
	tree.AddColumn().AddText(config.Node.Hostname).SetColor(colors.Bold)
	tree.AddColumn()
	tree.AddColumn().AddText(t.Avail.String())
	tree.AddColumn().AddText(fmt.Sprint(t.Priority)).SetColor(colors.Secondary)
	for rid, r := range t.Resources {
		n := tree.AddNode()
		n.AddColumn().AddText(rid)
		n.AddColumn().AddText("") // flags
		n.AddColumn().AddText(r.Status.String()).SetColor(colors.Primary)
		n.AddColumn().AddText(r.Label)
	}
	return tree.Render()
}
