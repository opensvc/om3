package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/key"
)

func (a *DaemonAPI) GetObjectKVStoreKeys(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	log := LogHandler(ctx, "GetObjectKVStore")

	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}

	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	instanceConfigData := instance.ConfigData.GetByPath(p)

	if _, ok := instanceConfigData[a.localhost]; ok {
		ks, err := object.NewKeystore(p)

		switch {
		case errors.Is(err, object.ErrWrongType):
			return JSONProblemf(ctx, http.StatusBadRequest, "NewKeystore", "%s", err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewKeystore", "%s", err)
		}

		if names, err := ks.AllKeys(); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Keys", "%s", err)
		} else {
			items := make(api.KVStoreKeyListItems, 0)
			for _, name := range names {
				configKey := key.T{
					Section: "data",
					Option:  name,
				}
				size := len(ks.Config().GetString(configKey))
				items = append(items, api.KVStoreKeyListItem{
					Object: p.String(),
					Node:   a.localhost,
					Key:    name,
					Size:   size,
				})
			}
			return ctx.JSON(http.StatusOK, api.KVStoreKeyList{
				Kind:  "KVStoreKeyList",
				Items: items,
			})
		}
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.GetObjectKVStoreKeysWithResponse(ctx.Request().Context(), namespace, kind, name); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
