package daemonapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonAPI) GetNodeConfigFile(ctx echo.Context, nodename string) error {
	if v, err := assertRole(ctx, rbac.RoleRoot); !v {
		return err
	}
	if a.localhost == nodename {
		logName := "GetNodeConfigFile"
		log := LogHandler(ctx, logName)
		log.Debugf("%s: starting", logName)

		filename := rawconfig.NodeConfigFile()
		mtime := file.ModTime(filename)
		if !mtime.IsZero() {
			ctx.Response().Header().Add(api.HeaderLastModifiedNano, mtime.Format(time.RFC3339Nano))
			log.Infof("serve node config file to %s", userFromContext(ctx).GetUserName())
			return ctx.File(filename)
		}
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "Node config file not found")
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeConfigFile(ctx.Request().Context(), nodename)
	})
}
