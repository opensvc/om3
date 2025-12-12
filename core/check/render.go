package check

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/render/tree"
)

// Render returns a human friendly string representation of the type.
func (t ResultSet) Render() string {
	tree := tree.New()
	tree.AddColumn().AddText(hostname.Hostname()).SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("driver")
	tree.AddColumn().AddText("instance")
	tree.AddColumn().AddText("object")
	tree.AddColumn().AddText("value")
	tree.AddColumn().AddText("unit")
	for _, r := range t.Data {
		n := tree.AddNode()
		n.AddColumn().AddText(r.DriverGroup).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(r.DriverName).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(r.Instance).SetColor(rawconfig.Color.Secondary)
		n.AddColumn().AddText(r.Path)
		n.AddColumn().AddText(fmt.Sprintf("%d", r.Value))
		n.AddColumn().AddText(r.Unit)
	}
	return tree.Render()
}
