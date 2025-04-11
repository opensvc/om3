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

func (a *DaemonAPI) PatchObjectKVStore(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	log := LogHandler(ctx, "PatchObjectKVStore")

	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}

	var (
		patches api.PatchKVStoreEntries
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

	getBytes := func(patch api.PatchKVStoreEntry) ([]byte, error) {
		switch {
		case patch.Bytes == nil && patch.String == nil:
			return nil, JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "bytes or string is required to add or change key %s", patch.Key)
		case patch.Bytes != nil && patch.String != nil:
			return nil, JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "only one of bytes or string is allowed to add or change key %s", patch.Key)
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
		ks, err := object.NewKVStore(p)

		switch {
		case errors.Is(err, object.ErrWrongType):
			return JSONProblemf(ctx, http.StatusBadRequest, "NewKVStore", "%s", err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewKVStore", "%s", err)
		}

		for _, patch := range patches {
			switch patch.Action {
			case "add":
				b, err := getBytes(patch)
				if err != nil {
					return err
				}
				if err := ks.TransactionAddKey(patch.Key, b); err != nil {
					return JSONProblemf(ctx, http.StatusInternalServerError, "AddKey", "%s: %s", patch.Key, err)
				}
			case "change":
				b, err := getBytes(patch)
				if err != nil {
					return err
				}
				if err := ks.TransactionChangeKey(patch.Key, b); err != nil {
					return JSONProblemf(ctx, http.StatusInternalServerError, "ChangeKey", "%s: %s", patch.Key, err)
				}
			case "remove":
				if err := ks.TransactionRemoveKey(patch.Key); err != nil {
					return JSONProblemf(ctx, http.StatusInternalServerError, "RemoveKey", "%s: %s", patch.Key, err)
				}
			case "rename":
				if patch.Name == nil {
					JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s: rename with no target name", patch.Key)
				}
				if err := ks.TransactionRenameKey(patch.Key, *patch.Name); err != nil {
					return JSONProblemf(ctx, http.StatusInternalServerError, "RenameKey", "%s: %s", patch.Key, err)
				}
			default:
				return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s: action %s is not supported, use add, change or remove", patch.Key, patch.Action)
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
		if resp, err := c.PatchObjectKVStoreWithResponse(ctx.Request().Context(), namespace, kind, name, patches); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
