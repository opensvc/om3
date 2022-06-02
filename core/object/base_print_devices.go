package object

import (
	"strings"

	"opensvc.com/opensvc/core/objectdevice"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/device"
)

type (
	// OptsPrintDevices is the options of the PrintDevices object method.
	OptsPrintDevices struct {
		Global OptsGlobal
		Roles  string `flag:"devroles"`
	}
	devExposer interface {
		ExposedDevices() []*device.T
	}
	devUser interface {
		SubDevices() []*device.T
	}
	devBaser interface {
		BaseDevices() []*device.T
	}
	devClaimer interface {
		ClaimedDevices() []*device.T
	}
)

func (t *Base) newObjectdevice(dev *device.T, role objectdevice.Role, r resource.Driver) objectdevice.T {
	return objectdevice.T{
		Device:     dev,
		Role:       role,
		RID:        r.RID(),
		DriverID:   r.Manifest().DriverID,
		ObjectPath: t.Path,
	}
}

// PrintDevices display the object base, sub and exposed devices
func (t *Base) PrintDevices(options OptsPrintDevices) objectdevice.L {
	var exposed, sub, base, claimed bool
	m := make(map[string]interface{})
	for _, role := range strings.Split(options.Roles, ",") {
		m[role] = nil
	}
	if _, ok := m["all"]; ok {
		return t.printDevices(true, true, true, true)
	}
	if _, ok := m["exposed"]; ok {
		exposed = true
	}
	if _, ok := m["sub"]; ok {
		sub = true
	}
	if _, ok := m["base"]; ok {
		base = true
	}
	if _, ok := m["claimed"]; ok {
		claimed = true
	}
	return t.printDevices(exposed, sub, base, claimed)
}

func (t *Base) printDevices(exposed, sub, base, claimed bool) objectdevice.L {
	l := objectdevice.NewList()
	for _, r := range t.Resources() {
		var i interface{} = r
		if exposed {
			if o, ok := i.(devExposer); ok {
				for _, dev := range o.ExposedDevices() {
					l = l.Add(t.newObjectdevice(dev, objectdevice.RoleExposed, r))
				}
			}
		}
		if sub {
			if o, ok := i.(devUser); ok {
				for _, dev := range o.SubDevices() {
					l = l.Add(t.newObjectdevice(dev, objectdevice.RoleSub, r))
				}
			}
		}
		if base {
			if o, ok := i.(devBaser); ok {
				for _, dev := range o.BaseDevices() {
					l = l.Add(t.newObjectdevice(dev, objectdevice.RoleBase, r))
				}
			}
		}
		if claimed {
			if o, ok := i.(devClaimer); ok {
				for _, dev := range o.ClaimedDevices() {
					l = l.Add(t.newObjectdevice(dev, objectdevice.RoleClaimed, r))
				}
			}
		}
	}
	return l
}
