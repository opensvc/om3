package daemonapi

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectdevice"
	"github.com/opensvc/om3/v3/daemon/api"
)

// GetInstanceDevices answers the devices of the resources of the instance on
// the node: a read, as the resource info of an instance.
func (a *DaemonAPI) GetInstanceDevices(ctx echo.Context, nodename api.InPathNodeName, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.GetInstanceDevicesParams) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetInstanceDevices(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	if !p.Exists() {
		return JSONProblemf(ctx, http.StatusNotFound, "No local instance", "%s has no instance on this node", p)
	}
	type devicer interface {
		PrintDevices(context.Context, objectdevice.Role) objectdevice.L
	}
	o, err := object.New(p, object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s", err)
	}
	roles := "all"
	if params.Roles != nil && *params.Roles != "" {
		roles = *params.Roles
	}
	l := objectdevice.NewList()
	if i, ok := o.(devicer); ok {
		l = i.PrintDevices(ctx.Request().Context(), objectdevice.ParseRoles(roles))
	}
	return ctx.JSON(http.StatusOK, a.instanceDeviceList(l))
}

// instanceDeviceList returns the devices as the api answers them.
func (a *DaemonAPI) instanceDeviceList(l objectdevice.L) api.InstanceDeviceList {
	r := api.InstanceDeviceList{
		Kind:  api.InstanceDeviceListKindInstanceDeviceList,
		Items: make(api.InstanceDeviceItems, len(l)),
	}
	for i, e := range l {
		r.Items[i] = api.InstanceDeviceItem{
			Kind: "InstanceDeviceItem",
			Meta: api.InstanceMeta{Node: a.localhost, Object: e.ObjectPath.String()},
			Data: api.InstanceDevice{
				Path:   e.ObjectPath.String(),
				Rid:    e.RID,
				Driver: api.InstanceDeviceDriver{Group: e.DriverID.Group.String(), Name: e.DriverID.Name},
				Role:   api.InstanceDeviceRole(e.Role.String()),
				Device: e.Device.Path(),
			},
		}
	}
	return r
}
