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

func (a *DaemonAPI) DeleteObjectKVStoreEntry(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.DeleteObjectKVStoreEntryParams) error {
	log := LogHandler(ctx, "DeleteObjectKVStoreEntry")

	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}

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

		err = ks.RemoveKey(params.Key)
		switch {
		case errors.Is(err, object.ErrKeyNotExist):
			return ctx.NoContent(http.StatusNoContent)
		case errors.Is(err, object.ErrKeyEmpty):
			return JSONProblemf(ctx, http.StatusBadRequest, "RemoveKey", "%s: %s", params.Key, err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "RemoveKey", "%s: %s", params.Key, err)
		default:
			return ctx.NoContent(http.StatusNoContent)
		}
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.DeleteObjectKVStoreEntryWithResponse(ctx.Request().Context(), namespace, kind, name, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
