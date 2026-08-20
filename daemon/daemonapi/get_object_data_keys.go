package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/key"
)

func (a *DaemonAPI) GetObjectDataKeys(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetObjectDataKeysParams) error {
	log := LogHandler(ctx, "GetObjectData")

	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}

	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	var filter *string
	if params.Filter != nil {
		filter = params.Filter
	}

	instanceConfigData := instance.ConfigData.GetByPath(p)

	if _, ok := instanceConfigData[a.localhost]; ok {
		ks, err := object.NewDataStore(p)

		switch {
		case errors.Is(err, object.ErrWrongType):
			return JSONProblemf(ctx, http.StatusBadRequest, "NewDataStore", "%s", err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewDataStore", "%s", err)
		}

		var names []string
		var keysErr error
		if filter != nil && *filter != "" {
			names, keysErr = ks.MatchingKeys(*filter)
		} else {
			names, keysErr = ks.AllKeys()
		}
		if keysErr != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Keys", "%s", keysErr)
		} else {
			items := make(api.DataKeyListItems, 0)
			for _, name := range names {
				configKey := key.T{
					Section: "data",
					Option:  name,
				}
				size := len(ks.Config().GetString(configKey))
				items = append(items, api.DataKeyListItem{
					Object: p.String(),
					Node:   a.localhost,
					Name:   name,
					Size:   size,
				})
			}
			return ctx.JSON(http.StatusOK, api.DataKeyList{
				Kind:  "DataKeyList",
				Items: items,
			})
		}
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.GetObjectDataKeysWithResponse(ctx.Request().Context(), namespace, kind, name, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
