package daemonapi

import (
	"errors"
	"net/http"
	"unicode/utf8"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetObjectDataStoreKey(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetObjectDataStoreKeyParams) error {
	log := LogHandler(ctx, "GetObjectDataStoreKey")

	if kind == naming.KindSec {
		if v, err := assertAdmin(ctx, namespace); !v {
			return err
		}
	} else {
		if v, err := assertGuest(ctx, namespace); !v {
			return err
		}
	}

	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	instanceConfigData := instance.ConfigData.GetByPath(p)

	if _, ok := instanceConfigData[a.localhost]; ok {
		ks, err := object.NewDataStore(p)

		switch {
		case errors.Is(err, object.ErrWrongType):
			return JSONProblemf(ctx, http.StatusBadRequest, "NewDataStore", "%s", err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewDataStore", "%s", err)
		}

		b, err := ks.DecodeKey(params.Name)
		switch {
		case err == nil:
			var contentType string
			if utf8.Valid(b) {
				contentType = "text/plain"
			} else {
				contentType = "application/octet-stream"
			}
			return ctx.Blob(http.StatusOK, contentType, b)
		case errors.Is(err, object.ErrKeyEmpty):
			return JSONProblemf(ctx, http.StatusBadRequest, "DecodeKey", "%s", err)
		case errors.Is(err, object.ErrKeyNotExist):
			return JSONProblemf(ctx, http.StatusNotFound, "DecodeKey", "%s", err)
		default:
			return JSONProblemf(ctx, http.StatusInternalServerError, "DecodeKey", "%s: %s", params.Name, err)
		}
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.GetObjectDataStoreKeyWithResponse(ctx.Request().Context(), namespace, kind, name, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
