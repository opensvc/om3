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

func (a *DaemonAPI) PatchObjectData(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	log := LogHandler(ctx, "PatchObjectData")

	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}

	var (
		patches api.PatchDataKeys
	)

	if err := ctx.Bind(&patches); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "error: %s", err)
	}

	if len(patches) == 0 {
		return ctx.NoContent(http.StatusNoContent)
	}

	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	instanceConfigData := instance.ConfigData.GetByPath(p)

	getBytes := func(patch api.PatchDataKey) ([]byte, error) {
		switch {
		case patch.Bytes == nil && patch.String == nil:
			return nil, JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "bytes or string is required to add or change key %s", patch.Name)
		case patch.Bytes != nil && patch.String != nil:
			return nil, JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "only one of bytes or string is allowed to add or change key %s", patch.Name)
		case patch.Bytes != nil:
			return *patch.Bytes, nil
		case patch.String != nil:
			return []byte(*patch.String), nil
		default:
			// no way to get here, just to please the builder
			return nil, nil
		}
	}

	if _, ok := instanceConfigData[a.localhost]; ok {
		ks, err := object.NewDataStore(p)

		switch {
		case errors.Is(err, object.ErrWrongType):
			return JSONProblemf(ctx, http.StatusBadRequest, "NewDataStore", "%s", err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewDataStore", "%s", err)
		}

		for _, patch := range patches {
			switch patch.Action {
			case "add":
				b, err := getBytes(patch)
				if err != nil {
					return err
				}
				if err := ks.TransactionAddKey(patch.Name, b); err != nil {
					status := http.StatusInternalServerError
					if errors.Is(err, object.ErrValueTooBig) {
						status = http.StatusRequestEntityTooLarge
					}
					return JSONProblemf(ctx, status, "AddKey", "%s: %s", patch.Name, err)
				}
			case "change":
				b, err := getBytes(patch)
				if err != nil {
					return err
				}
				if err := ks.TransactionChangeKey(patch.Name, b); err != nil {
					status := http.StatusInternalServerError
					if errors.Is(err, object.ErrValueTooBig) {
						status = http.StatusRequestEntityTooLarge
					}
					return JSONProblemf(ctx, status, "ChangeKey", "%s: %s", patch.Name, err)
				}
			case "remove":
				if err := ks.TransactionRemoveKey(patch.Name); err != nil {
					return JSONProblemf(ctx, http.StatusInternalServerError, "RemoveKey", "%s: %s", patch.Name, err)
				}
			case "rename":
				if patch.To == nil {
					JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s: rename with no target name", patch.Name)
				}
				if err := ks.TransactionRenameKey(patch.Name, *patch.To); err != nil {
					return JSONProblemf(ctx, http.StatusInternalServerError, "RenameKey", "%s: %s", patch.Name, err)
				}
			default:
				return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s: action %s is not supported, use add, change or remove", patch.Name, patch.Action)
			}
		}

		if err := ks.Config().CommitInvalid(); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Commit", "%s", err)
		}
		return ctx.NoContent(http.StatusNoContent)
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.PatchObjectDataWithResponse(ctx.Request().Context(), namespace, kind, name, patches); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
