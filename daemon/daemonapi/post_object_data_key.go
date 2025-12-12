package daemonapi

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

func (a *DaemonAPI) PostObjectDataKey(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectDataKeyParams) error {
	log := LogHandler(ctx, "PostObjectDataKey")

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
		ks, err := object.NewDataStore(p)

		switch {
		case errors.Is(err, object.ErrWrongType):
			return JSONProblemf(ctx, http.StatusBadRequest, "NewDataStore", "%s", err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewDataStore", "%s", err)
		}

		ok, err := a.CheckDataSize(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		b, err := ioutil.ReadAll(ctx.Request().Body)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "ReadAll", "%s: %s", params.Name, err)
		}
		err = ks.AddKey(params.Name, b)
		switch {
		case errors.Is(err, object.ErrKeyExist):
			return JSONProblemf(ctx, http.StatusConflict, "AddKey", "%s: %s. consider using the PUT method.", params.Name, err)
		case errors.Is(err, object.ErrKeyEmpty):
			return JSONProblemf(ctx, http.StatusBadRequest, "AddKey", "%s: %s", params.Name, err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "AddKey", "%s: %s", params.Name, err)
		default:
			return ctx.NoContent(http.StatusNoContent)
		}
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.PostObjectDataKeyWithBodyWithResponse(ctx.Request().Context(), namespace, kind, name, &params, "application/octet-stream", ctx.Request().Body); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}

func (a *DaemonAPI) CheckDataSize(ctx echo.Context) (bool, error) {
	contentLength := ctx.Request().Header.Get("Content-Length")
	if contentLength != "" {
		nodeData := node.ConfigData.GetByNode(a.localhost)
		if nodeData == nil {
			return false, JSONProblemf(ctx, http.StatusInternalServerError, "NodeConfig", "no config found for node %s", hostname.Hostname())
		}
		maxSize := nodeData.MaxKeySize
		if size, err := strconv.Atoi(contentLength); err == nil && size > int(maxSize) {
			return false, JSONProblemf(ctx, http.StatusRequestEntityTooLarge, "Body too large", "The request body exceeds the allowed size.")
		}
	}
	return true, nil
}
