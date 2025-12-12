package objectdevice

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/render/tree"
)

func (t L) Render() string {
	tree := tree.New()
	tree.AddColumn().AddText("Object").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Resource").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Driver").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Role").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Device").SetColor(rawconfig.Color.Bold)
	for _, e := range t {
		did := driver.NewID(e.DriverID.Group, e.DriverID.Name)
		n := tree.AddNode()
		n.AddColumn().AddText(e.ObjectPath.String()).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(e.RID).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(did.String()).SetColor(rawconfig.Color.Secondary)
		n.AddColumn().AddText(e.Role.String()).SetColor(rawconfig.Color.Secondary)
		n.AddColumn().AddText(e.Device.Path())
	}
	return tree.Render()
}
