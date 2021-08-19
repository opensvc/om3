package objectdevice

import (
	"opensvc.com/opensvc/core/driverid"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/render/tree"
)

func (t L) Render() string {
	tree := tree.New()
	tree.AddColumn().AddText("Object").SetColor(rawconfig.Node.Color.Bold)
	tree.AddColumn().AddText("Resource").SetColor(rawconfig.Node.Color.Bold)
	tree.AddColumn().AddText("Driver").SetColor(rawconfig.Node.Color.Bold)
	tree.AddColumn().AddText("Role").SetColor(rawconfig.Node.Color.Bold)
	tree.AddColumn().AddText("Device").SetColor(rawconfig.Node.Color.Bold)
	for _, e := range t {
		if e.Device == nil {
			// volume with no device
			continue
		}
		did := driverid.New(e.DriverGroup, e.DriverName)
		n := tree.AddNode()
		n.AddColumn().AddText(e.ObjectPath.String()).SetColor(rawconfig.Node.Color.Primary)
		n.AddColumn().AddText(e.RID).SetColor(rawconfig.Node.Color.Primary)
		n.AddColumn().AddText(did.String()).SetColor(rawconfig.Node.Color.Secondary)
		n.AddColumn().AddText(e.Role.String()).SetColor(rawconfig.Node.Color.Secondary)
		n.AddColumn().AddText(e.Device.Path())
	}
	return tree.Render()
}
