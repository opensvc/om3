package daemonapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/drbd"
)

func (a *DaemonAPI) DeleteNodeDRBDConfig(ctx echo.Context, nodename string, params api.DeleteNodeDRBDConfigParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.deleteLocalDRBDConfig(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.DeleteNodeDRBDConfig(ctx.Request().Context(), nodename, &params)
	})
}

// deleteLocalDRBDConfig removes the drbd configuration file the provisioning
// leader sent this node, when the resource never provisioned here: it is
// not up, and its provisioned state, the truth of what this node
// provisioned, does not say it is. A resource provisioned and stopped, or
// one whose disk is not reachable right now, keeps its file.
func (a *DaemonAPI) deleteLocalDRBDConfig(ctx echo.Context, params api.DeleteNodeDRBDConfigParams) error {
	if !isDRBDResName(params.Name) {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid parameter", "The 'name' parameter must be a basename.")
	}
	if !isDiskRID(params.Rid) {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid parameter", "The 'rid' parameter must be the id of a disk resource, as disk#1.")
	}
	p, err := naming.ParsePath(params.Path)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "path: %s", err)
	}
	cf := drbd.ResConfigFile(params.Name)
	if _, err := os.Stat(cf); errors.Is(err, os.ErrNotExist) {
		return ctx.JSON(http.StatusOK, nil)
	}
	if v, err := drbd.New(params.Name).IsDefined(ctx.Request().Context()); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Error reading the drbd resource state", "%s", err)
	} else if v {
		return JSONProblemf(ctx, http.StatusConflict, "Kept", "the drbd resource %s is up on %s", params.Name, a.localhost)
	}
	if v, err := isResourceProvisioned(p, params.Rid); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Error reading the provisioned state", "%s", err)
	} else if v {
		return JSONProblemf(ctx, http.StatusConflict, "Kept", "%s %s is provisioned on %s", p, params.Rid, a.localhost)
	}
	if err := os.Remove(cf); err != nil && !errors.Is(err, os.ErrNotExist) {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Error removing the drbd res file", "%s", err)
	}
	LogHandler(ctx, "DeleteNodeDRBDConfig").Infof("removed %s, sent for %s %s which never provisioned on this node", cf, p, params.Rid)
	return ctx.JSON(http.StatusOK, nil)
}

// isDiskRID says s is the id of a disk resource, as disk#1. It names a
// directory of the object var directory, which isResourceProvisioned reads
// from: a resource id is made of letters, digits and "_-.#" only, with no
// path separator to lead out of it.
func isDiskRID(s string) bool {
	rid, err := resourceid.Parse(s)
	return err == nil && rid.IsIndexed() && rid.DriverGroup() == driver.GroupDisk
}

// isResourceProvisioned reads the provisioned state the actions of the
// resource rid of the object p wrote on this node. No state, as for an
// instance never provisioned here or one already deleted, is not
// provisioned.
func isResourceProvisioned(p naming.Path, rid string) (bool, error) {
	b, err := os.ReadFile(filepath.Join(p.VarDir(), rid, "provisioned"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	var v bool
	if err := json.Unmarshal(b, &v); err != nil {
		return false, err
	}
	return v, nil
}
