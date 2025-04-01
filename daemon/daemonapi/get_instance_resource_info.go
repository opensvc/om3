package daemonapi

import (
	"errors"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetInstanceResourceInfo(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.getLocalInstanceResourceInfo(ctx, namespace, kind, name)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetInstanceResourceInfo(ctx.Request().Context(), nodename, namespace, kind, name)
	})
}

func (a *DaemonAPI) getLocalInstanceResourceInfo(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	path, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New path", "%s", err)
	}
	if !path.Exists() {
		return JSONProblemf(ctx, http.StatusNotFound, "No local instance", "")
	}

	type loadResInfoer interface {
		LoadResInfo() (resource.Infos, error)
	}

	o, err := object.New(path)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s", err)
	}

	i, ok := o.(loadResInfoer)
	if !ok {
		return JSONProblemf(ctx, http.StatusBadRequest, "Load info", "Object does not support info: %s", path)
	}

	resp := api.ResourceInfoList{
		Kind:  "ResourceInfoList",
		Items: api.ResourceInfoItems{},
	}

	infos, err := i.LoadResInfo()
	if errors.Is(err, os.ErrNotExist) {
		return JSONProblemf(ctx, http.StatusNotFound, "Load resource info", "%s", err)
	} else if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Load resource info", "%s", err)
	}

	for _, r := range infos.Resources {
		for _, e := range r.Keys {
			item := api.ResourceInfoItem{
				Node:   a.localhost,
				Object: path.String(),
				Rid:    r.RID,
				Key:    e.Key,
				Value:  e.Value,
			}
			resp.Items = append(resp.Items, item)
		}
	}
	return ctx.JSON(http.StatusOK, resp)
}
