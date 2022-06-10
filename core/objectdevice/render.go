package objectdevice

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/render/tree"
)

func (t L) Render() string {
	tree := tree.New()
	tree.AddColumn().AddText("Object").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Resource").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Driver").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Role").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Device").SetColor(rawconfig.Color.Bold)
	for _, e := range t {
		if e.Device == nil {
			// volume with no device
			continue
		}
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
