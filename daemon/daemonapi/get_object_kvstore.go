package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetObjectKVStore(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetObjectKVStoreParams) error {
	log := LogHandler(ctx, "GetObjectKVStore")

	if kind == naming.KindSec {
		if v, err := assertAdmin(ctx, namespace); !v {
			return err
		}
	} else {
		if v, err := assertGuest(ctx, namespace); !v {
			return err
		}
	}

	result := make(api.KVStoreEntries, 0)

	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	instanceConfigData := instance.ConfigData.GetByPath(p)

	if _, ok := instanceConfigData[a.localhost]; ok {
		ks, err := object.NewKVStore(p)

		switch {
		case errors.Is(err, object.ErrWrongType):
			return JSONProblemf(ctx, http.StatusBadRequest, "NewKVStore", "%s", err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewKVStore", "%s", err)
		}

		if params.Keys == nil {
			return ctx.JSON(http.StatusOK, result)
		}

		for _, key := range *params.Keys {
			if b, err := ks.DecodeKey(key); err != nil {
				return JSONProblemf(ctx, http.StatusInternalServerError, "DecodeKey", "%s: %s", key, err)
			} else {
				result = append(result, api.KVStoreEntry{Key: key, Bytes: b})
			}
		}
		return ctx.JSON(http.StatusOK, result)
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.GetObjectKVStoreWithResponse(ctx.Request().Context(), namespace, kind, name, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
