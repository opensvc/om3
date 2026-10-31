package daemonapi

import (
	"errors"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/volfile"
)

// volumeHead returns the mount point of the local instance of a volume, or
// answers why there is none.
func (a *DaemonAPI) volumeHead(ctx echo.Context, namespace string, kind naming.Kind, name string) (string, error) {
	if kind != naming.KindVol {
		return "", JSONProblemf(ctx, http.StatusBadRequest, "Not a volume", "%s objects hold no file", kind)
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return "", JSONProblemf(ctx, http.StatusBadRequest, "Invalid path", "%s", err)
	}
	st := instance.StatusData.GetByPathAndNode(p, a.localhost)
	if st == nil {
		return "", JSONProblemf(ctx, http.StatusNotFound, "No local instance", "")
	}
	if !vpath.Accessible(st.Avail) {
		return "", JSONProblemf(ctx, http.StatusConflict, "Volume not available", "the %s instance is %s on %s", p, st.Avail, a.localhost)
	}
	vol, err := object.NewVol(p)
	if err != nil {
		return "", JSONProblemf(ctx, http.StatusInternalServerError, "New vol", "%s", err)
	}
	head := vol.Head()
	if head == "" {
		return "", JSONProblemf(ctx, http.StatusConflict, "No mount point", "%s holds no filesystem: it was provisioned with format=false", p)
	}
	return head, nil
}

// noVolumeHead says volumeHead answered the request itself.
func noVolumeHead(ctx echo.Context) bool {
	return ctx.Response().Committed
}

// GetInstanceFiles lists the files at the top of a volume.
func (a *DaemonAPI) GetInstanceFiles(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetInstanceFiles(ctx.Request().Context(), nodename, namespace, kind, name)
		})
	}
	head, err := a.volumeHead(ctx, namespace, kind, name)
	if noVolumeHead(ctx) {
		return err
	}
	l, err := volfile.List(head)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "List files", "%s", err)
	}
	items := make([]api.VolumeFile, len(l))
	for i, f := range l {
		items[i] = api.VolumeFile{Name: f.Name, Size: f.Size, ModifiedAt: f.ModifiedAt}
	}
	return ctx.JSON(http.StatusOK, api.VolumeFileList{Kind: "VolumeFileList", Items: items})
}

// PostInstanceFile issues the ticket a file of a volume is uploaded with.
//
// The api decides who may upload, and the transfer is not served through it:
// the ticket is presented to the console listener, which hands the connection
// to a process of its own. A transfer lasts as long as its client takes, which
// is no reason to have the daemon decrypt it, nor to lose it when it restarts.
func (a *DaemonAPI) PostInstanceFile(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceFileParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	if !volfile.IsName(params.Name) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid file name", "%q: a name at the top of the volume is expected", params.Name)
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostInstanceFile(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	// Said before a transfer is started, rather than at its end.
	_, err := a.volumeHead(ctx, namespace, kind, name)
	if noVolumeHead(ctx) {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid path", "%s", err)
	}
	return a.issueConsoleTicket(ctx, console.Target{Node: a.localhost, Path: p.String(), Kind: console.KindUpload, File: params.Name})
}

// DeleteInstanceFile removes a file of a volume.
func (a *DaemonAPI) DeleteInstanceFile(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.DeleteInstanceFileParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	if !volfile.IsName(params.Name) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid file name", "%q: a name at the top of the volume is expected", params.Name)
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.DeleteInstanceFile(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	head, err := a.volumeHead(ctx, namespace, kind, name)
	if noVolumeHead(ctx) {
		return err
	}
	err = volfile.Remove(head, params.Name)
	switch {
	case err == nil:
		return ctx.NoContent(http.StatusNoContent)
	case errors.Is(err, fs.ErrNotExist):
		return JSONProblemf(ctx, http.StatusNotFound, "No such file", "%s", params.Name)
	default:
		return JSONProblemf(ctx, http.StatusInternalServerError, "Remove file", "%s", err)
	}
}
