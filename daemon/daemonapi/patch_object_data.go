package daemonapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/pubsub"
)

func (a *DaemonAPI) PatchObjectData(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PatchObjectDataParams) error {
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

	waitCtx, cancel, waiting, err := waitContext(ctx, params.Wait)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	defer cancel()

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
		if v, err := assertUsrKeyWrite(ctx, p); !v {
			return err
		}
		ks, err := object.NewDataStore(p)

		switch {
		case errors.Is(err, object.ErrWrongType):
			return JSONProblemf(ctx, http.StatusBadRequest, "NewDataStore", "%s", err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewDataStore", "%s", err)
		}

		var sub *pubsub.Subscription
		if waiting {
			sub = a.subscribeConfigPropagation(fmt.Sprintf("api.patch_object_data %s %s", p, ctx.Get("uuid")), p)
			defer func() { _ = sub.Stop() }()
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
					return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s: rename with no target name", patch.Name)
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
		a.announceConfigFileWritten(p)
		mtime := file.ModTime(p.ConfigFile())
		if !mtime.IsZero() {
			ctx.Response().Header().Add(api.HeaderLastModified, mtime.Format(time.RFC3339Nano))
		}
		// Asked to wait, the answer is held until the configuration has
		// reached every live node of the object, as a configuration update
		// does: a wait that expires says where it has not landed yet, and is
		// not a failure of the write.
		if waiting && !mtime.IsZero() {
			if lagging := a.waitConfigPropagated(waitCtx, sub, p, mtime); len(lagging) > 0 {
				return JSONProblemf(ctx, http.StatusRequestTimeout, "Configuration still propagating",
					"the keys of %s are committed as of %s, and have not reached %s before the wait expired",
					p, mtime.Format(time.RFC3339Nano), strings.Join(lagging, ", "))
			}
		}
		return ctx.NoContent(http.StatusNoContent)
	}

	// The operations are applied by the first node holding the object, which
	// answers for them: the configuration it commits reaches the others.
	for nodename := range instanceConfigData {
		var proxyOpts []funcopt.O
		if waiting {
			// The peer holds the answer for as long as the wait, which is
			// longer than a client allows an answer by default.
			proxyOpts = append(proxyOpts, client.WithTimeout(0))
		}
		c, err := a.newProxyClient(ctx, nodename, proxyOpts...)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		resp, err := c.PatchObjectDataWithResponse(ctx.Request().Context(), namespace, kind, name, &params, patches)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		}
		if v := resp.HTTPResponse.Header.Get(api.HeaderLastModified); v != "" {
			ctx.Response().Header().Add(api.HeaderLastModified, v)
		}
		if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
		return ctx.NoContent(resp.StatusCode())
	}

	return JSONProblemf(ctx, http.StatusNotFound, "Not found", "object not found: %s", p)
}
