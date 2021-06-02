package check

import (
	"fmt"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/render/tree"
)

// Render returns a human friendly string representation of the type.
func (t ResultSet) Render() string {
	tree := tree.New()
	tree.AddColumn().AddText(hostname.Hostname()).SetColor(config.Node.Color.Bold)
	tree.AddColumn().AddText("driver")
	tree.AddColumn().AddText("instance")
	tree.AddColumn().AddText("value")
	tree.AddColumn().AddText("unit")
	for _, r := range t.Data {
		n := tree.AddNode()
		n.AddColumn().AddText(r.DriverGroup).SetColor(config.Node.Color.Primary)
		n.AddColumn().AddText(r.DriverName).SetColor(config.Node.Color.Primary)
		n.AddColumn().AddText(r.Instance).SetColor(config.Node.Color.Secondary)
		n.AddColumn().AddText(fmt.Sprintf("%d", r.Value))
		n.AddColumn().AddText(r.Unit)
	}
	return tree.Render()
}
