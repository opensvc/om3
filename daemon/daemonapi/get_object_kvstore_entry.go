package daemonapi

import (
	"net/http"
	"unicode/utf8"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonAPI) GetObjectKVStoreEntry(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetObjectKVStoreEntryParams) error {
	log := LogHandler(ctx, "GetObjectKVStoreEntry")

	if v, err := assertGrant(ctx, rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot); !v {
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
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewKeystore", "%s", err)
		}

		if b, err := ks.DecodeKey(params.Key); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "DecodeKey", "%s: %s", params.Key, err)
		} else {
			var contentType string
			if utf8.Valid(b) {
				contentType = "text/plain"
			} else {
				contentType = "application/octet-stream"
			}
			return ctx.Blob(http.StatusOK, contentType, b)
		}
		return ctx.NoContent(http.StatusNoContent)
	}

	for nodename := range instanceConfigData {
		c, err := newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.GetObjectKVStoreEntryWithResponse(ctx.Request().Context(), namespace, kind, name, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
