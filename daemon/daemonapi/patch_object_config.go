package daemonapi

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/key"
)

func (a *DaemonAPI) PatchObjectConfig(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PatchObjectConfigParams) error {
	log := LogHandler(ctx, "patchObjectConfig")

	if kind == naming.KindNscfg {
		if v, err := assertNamespaceConfigWriter(ctx); !v {
			return err
		}
	} else if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}

	sets := make(keyop.L, 0)
	unsets := make(key.L, 0)
	deletes := make([]string, 0)

	if params.Set != nil {
		sets = keyop.ParseOps(*params.Set)
	}
	if params.Unset != nil {
		unsets = key.ParseStrings(*params.Unset)
	}
	if params.Delete != nil {
		for _, section := range *params.Delete {
			if section == "" {
				// Prevents from accidental remove DEFAULT section. SectionsByName("")
				// return "DEFAULT". Use explicit section="DEFAULT" to remove DEFAULT section.
				continue
			}
			deletes = append(deletes, section)
		}
	}
	if len(sets)+len(unsets)+len(deletes) == 0 {
		return JSONProblemf(ctx, http.StatusBadRequest, "No valid update requested", "")
	}

	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	// The size an object is configured to hold is the target a running resize
	// reads on every node, so changing it while one runs makes that resize
	// grow to a size its own request never named. The daemon is where the
	// write and the orchestration meet, so it is where this is caught: the
	// client cannot, its view of a monitor trailing the request that set it.
	if err := refuseSizeWhileResizing(p, sets); err != nil {
		return JSONProblemf(ctx, http.StatusConflict, "Resize in progress", "%s", err)
	}

	instanceConfigData := instance.ConfigData.GetByPath(p)

	if _, ok := instanceConfigData[a.localhost]; ok {
		changed, err := configUpdate(ctx, log, p, deletes, unsets, sets)
		if errors.Is(err, ErrDenied) {
			return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "%s", err)
		} else if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Update config", "%s", err)
		}
		// Answer with the timestamp the configuration now carries, as a
		// configuration file write does, so the caller can require it of what
		// it goes on to ask of the instances.
		if mtime := file.ModTime(p.ConfigFile()); !mtime.IsZero() {
			ctx.Response().Header().Add(api.HeaderLastModified, mtime.Format(time.RFC3339Nano))
		}
		return ctx.JSON(http.StatusOK, api.Committed{IsChanged: changed})
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			log.Warnf("new client for %s@%s: %s", p, nodename, err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.PatchObjectConfigWithResponse(ctx.Request().Context(), namespace, kind, name, &params); err != nil {
			log.Warnf("request proxy %s@%s: %s", p, nodename, err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else {
			log.Tracef("request proxy to %s for %s status: %s", nodename, p, resp.Status())
			if len(resp.Body) > 0 {
				return ctx.JSONBlob(resp.StatusCode(), resp.Body)
			} else {
				return ctx.NoContent(resp.StatusCode())
			}
		}
	}

	log.Tracef("can't patch: object not found %s", p)
	return JSONProblemf(ctx, http.StatusNotFound, "Not found", "object not found: %s", p)
}

// refuseSizeWhileResizing stops a write of the configured size of an object a
// resize orchestration is running on.
func refuseSizeWhileResizing(p naming.Path, sets keyop.L) error {
	setsSize := false
	for _, op := range sets {
		if op.Key.Option == "size" && (op.Key.Section == "" || op.Key.Section == "DEFAULT") {
			setsSize = true
			break
		}
	}
	if !setsSize {
		return nil
	}
	for nodename, instMon := range instance.MonitorData.GetByPath(p) {
		if instMon.GlobalExpect == instance.MonitorGlobalExpectResized {
			return fmt.Errorf("%s is already resizing, asked of %s: wait for it to end, or abort it with \"om %s abort\"", p, nodename, p)
		}
	}
	return nil
}
