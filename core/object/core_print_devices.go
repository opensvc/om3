package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/objectdevice"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/device"
)

type (
	devExposer interface {
		ExposedDevices(context.Context) device.L
	}
	devUser interface {
		SubDevices(context.Context) device.L
	}
	devBaser interface {
		BaseDevices(context.Context) device.L
	}
	devClaimer interface {
		ClaimedDevices(context.Context) device.L
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

func (t *actor) PrintDevices(ctx context.Context, roles objectdevice.Role) objectdevice.L {
	l := objectdevice.NewList()
	for _, r := range t.Resources() {
		var i interface{} = r
		if roles&objectdevice.RoleExposed != 0 {
			if o, ok := i.(devExposer); ok {
				for _, dev := range o.ExposedDevices(ctx) {
					l = l.Add(t.newObjectdevice(dev, objectdevice.RoleExposed, r))
				}
			}
		}
		if roles&objectdevice.RoleSub != 0 {
			if o, ok := i.(devUser); ok {
				for _, dev := range o.SubDevices(ctx) {
					l = l.Add(t.newObjectdevice(dev, objectdevice.RoleSub, r))
				}
			}
		}
		if roles&objectdevice.RoleBase != 0 {
			if o, ok := i.(devUser); ok {
				subDevs := o.SubDevices(ctx)
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
				for _, dev := range o.ClaimedDevices(ctx) {
					l = l.Add(t.newObjectdevice(dev, objectdevice.RoleClaimed, r))
				}
			}
		}
	}
	return l
}
