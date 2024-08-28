package daemonapi

import (
	"errors"
	"io/ioutil"
	"net/http"
	"slices"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonAPI) PutObjectKVStoreEntry(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PutObjectKVStoreEntryParams) error {
	log := LogHandler(ctx, "PutObjectKVStoreEntry")

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
		keys, err := ks.AllKeys()
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "AllKeys", "%s", err)
		}
		if !slices.Contains(keys, params.Key) {
			return JSONProblemf(ctx, http.StatusNotFound, "ChangeKey", "%s: %s. consider using the POST method.", params.Key, err)
		}
		b, err := ioutil.ReadAll(ctx.Request().Body)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "ReadAll", "%s: %s", params.Key, err)
		}
		err = ks.ChangeKey(params.Key, b)
		switch {
		case errors.Is(err, object.KeystoreErrKeyEmpty):
			return JSONProblemf(ctx, http.StatusBadRequest, "ChangeKey", "%s: %s", params.Key, err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "ChangeKey", "%s: %s", params.Key, err)
		default:
			return ctx.NoContent(http.StatusNoContent)
		}
	}

	for nodename := range instanceConfigData {
		c, err := newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.PutObjectKVStoreEntryWithBodyWithResponse(ctx.Request().Context(), namespace, kind, name, &params, "application/octet-stream", ctx.Request().Body); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
