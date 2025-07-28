package object

import (
	"github.com/opensvc/om3/core/objectdevice"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/util/device"
)

type (
	devExposer interface {
		ExposedDevices() device.L
	}
	devUser interface {
		SubDevices() device.L
	}
	devBaser interface {
		BaseDevices() device.L
	}
	devClaimer interface {
		ClaimedDevices() device.L
	}
)

func (t *actor) newObjectdevice(dev device.T, role objectdevice.Role, r resource.Driver) objectdevice.T {
	return objectdevice.T{
		Device:     dev,
		Role:       role,
		RID:        r.RID(),
		DriverID:   r.Manifest().DriverID,
		ObjectPath: t.path,
	}
}

func (t *actor) PrintDevices(roles objectdevice.Role) objectdevice.L {
	l := objectdevice.NewList()
	for _, r := range t.Resources() {
		var i interface{} = r
		if roles&objectdevice.RoleExposed != 0 {
			if o, ok := i.(devExposer); ok {
				for _, dev := range o.ExposedDevices() {
					l = l.Add(t.newObjectdevice(dev, objectdevice.RoleExposed, r))
				}
			}
		}
		if roles&objectdevice.RoleSub != 0 {
			if o, ok := i.(devUser); ok {
				for _, dev := range o.SubDevices() {
					l = l.Add(t.newObjectdevice(dev, objectdevice.RoleSub, r))
				}
			}
		}
		if roles&objectdevice.RoleBase != 0 {
			if o, ok := i.(devUser); ok {
				subDevs := o.SubDevices()
				baseDevs, err := subDevs.HolderEndpoints()
				if err == nil {
					for _, dev := range baseDevs {
						l = l.Add(t.newObjectdevice(dev, objectdevice.RoleBase, r))
					}
				}
			}
		}
		if roles&objectdevice.RoleClaimed != 0 {
			if o, ok := i.(devClaimer); ok {
				for _, dev := range o.ClaimedDevices() {
					l = l.Add(t.newObjectdevice(dev, objectdevice.RoleClaimed, r))
				}
			}
		}
	}
	return l
}
