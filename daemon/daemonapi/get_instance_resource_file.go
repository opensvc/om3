package daemonapi

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetInstanceResourceFile(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.GetInstanceResourceFileParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		logName := "GetInstanceResourceFile"
		log := LogHandler(ctx, logName)
		log.Tracef("%s: starting", logName)

		// Verify the object path is valid
		objPath, err := naming.NewPath(namespace, kind, name)
		if err != nil {
			log.Warnf("%s: %s", logName, err)
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
		}
		log = naming.LogWithPath(log, objPath)

		// Verify the instance exists
		instStatus := instance.StatusData.GetByPathAndNode(objPath, nodename)
		if instStatus == nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "instance status not found: %s@%s", objPath, nodename)
		}

		// The instance is not required to be up: the requester judges
		// which peer is the authority for the files. An instance up is,
		// and so is the source of a placement once the orchestration
		// stopped it, whose files are the version it stopped with, the
		// one the destination is to start with. The files served are the
		// ones the status of the instance lists, whatever its state.

		// Verify the resource exists
		resourceStatus, ok := instStatus.Resources[params.Rid]
		if !ok {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "resource not found in instance status: %s@%s: %s", objPath, nodename, params.Rid)
		}

		// Verify the requested file is exposed by the resource
		isFound := false
		for _, f := range resourceStatus.Files {
			if f.Name == params.Name {
				isFound = true
				break
			}
		}
		if !isFound {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "file not exposed by resource in instance status: %s@%s: %s: %s", objPath, nodename, params.Rid, params.Name)
		}

		info, err := os.Stat(params.Name)
		if errors.Is(err, os.ErrNotExist) {
			return JSONProblemf(ctx, http.StatusNotFound, "Not Found", "")
		} else if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Internal Server Error", "Stat %s: %s", params.Name, err)
		}
		uxInfo := info.Sys().(*syscall.Stat_t)

		ctx.Response().Header().Add(api.HeaderLastModified, info.ModTime().Format(time.RFC3339Nano))
		ctx.Response().Header().Add(api.HeaderPerm, fmt.Sprintf("%o", info.Mode()))
		ctx.Response().Header().Add(api.HeaderGroup, fmt.Sprint(uxInfo.Gid))
		ctx.Response().Header().Add(api.HeaderUser, fmt.Sprint(uxInfo.Uid))

		log.Infof("serve %s file %s to %s", params.Rid, params.Name, userFromContext(ctx).Username)
		return ctx.File(params.Name)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetInstanceResourceFile(ctx.Request().Context(), nodename, namespace, kind, name, &params)
	})
}
