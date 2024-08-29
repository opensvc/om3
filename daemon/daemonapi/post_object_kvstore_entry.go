package daemonapi

import (
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonAPI) PostObjectKVStoreEntry(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectKVStoreEntryParams) error {
	log := LogHandler(ctx, "PostObjectKVStoreEntry")

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

		switch {
		case errors.Is(err, object.ErrWrongType):
			return JSONProblemf(ctx, http.StatusBadRequest, "NewKeystore", "%s", err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewKeystore", "%s", err)
		}

		b, err := ioutil.ReadAll(ctx.Request().Body)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "ReadAll", "%s: %s", params.Key, err)
		}
		err = ks.AddKey(params.Key, b)
		switch {
		case errors.Is(err, object.KeystoreErrExist):
			return JSONProblemf(ctx, http.StatusConflict, "AddKey", "%s: %s. consider using the PUT method.", params.Key, err)
		case errors.Is(err, object.KeystoreErrKeyEmpty):
			return JSONProblemf(ctx, http.StatusBadRequest, "AddKey", "%s: %s", params.Key, err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "AddKey", "%s: %s", params.Key, err)
		default:
			return ctx.NoContent(http.StatusNoContent)
		}
	}

	for nodename := range instanceConfigData {
		c, err := newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.PostObjectKVStoreEntryWithBodyWithResponse(ctx.Request().Context(), namespace, kind, name, &params, "application/octet-stream", ctx.Request().Body); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
