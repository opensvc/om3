package object

import (
	"opensvc.com/opensvc/core/objectdevice"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/device"
)

type (
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

func (t *actor) newObjectdevice(dev *device.T, role objectdevice.Role, r resource.Driver) objectdevice.T {
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
			if o, ok := i.(devBaser); ok {
				for _, dev := range o.BaseDevices() {
					l = l.Add(t.newObjectdevice(dev, objectdevice.RoleBase, r))
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
