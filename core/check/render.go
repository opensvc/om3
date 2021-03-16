package check

import (
	"fmt"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/render/palette"
	"opensvc.com/opensvc/util/render/tree"
)

func (t ResultSet) Render() string {
	colors := palette.New(config.Node.Palette)
	tree := tree.New()
	tree.AddColumn().AddText(config.Node.Hostname).SetColor(colors.Bold)
	tree.AddColumn().AddText("driver")
	tree.AddColumn().AddText("instance")
	tree.AddColumn().AddText("value")
	tree.AddColumn().AddText("unit")
	for _, r := range t.Data {
		n := tree.AddNode()
		n.AddColumn().AddText(r.DriverGroup).SetColor(colors.Primary)
		n.AddColumn().AddText(r.DriverName).SetColor(colors.Primary)
		n.AddColumn().AddText(r.Instance).SetColor(colors.Secondary)
		n.AddColumn().AddText(fmt.Sprintf("%d", r.Value))
		n.AddColumn().AddText(r.Unit)
	}
	return tree.Render()
}
