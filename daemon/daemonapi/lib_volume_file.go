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

// volumeDir returns the directory of the volume a request names, cleaned, or
// answers why it is none.
func volumeDir(ctx echo.Context, dir *string) (string, error) {
	var s string
	if dir != nil {
		s = *dir
	}
	cleaned, err := volfile.CleanDir(s)
	if err != nil {
		return "", JSONProblemf(ctx, http.StatusBadRequest, "Invalid directory", "%s", err)
	}
	return cleaned, nil
}

// volfileProblem answers what a volfile function refused, or failed with.
func volfileProblem(ctx echo.Context, err error, what string) error {
	switch {
	case errors.Is(err, volfile.ErrInvalidDir):
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid directory", "%s", err)
	case errors.Is(err, volfile.ErrNoDir):
		return JSONProblemf(ctx, http.StatusNotFound, "No such directory", "%s", err)
	case errors.Is(err, volfile.ErrNotDir):
		return JSONProblemf(ctx, http.StatusConflict, "Not a directory", "%s", err)
	case errors.Is(err, volfile.ErrNotEmpty):
		return JSONProblemf(ctx, http.StatusConflict, "Directory not empty", "%s", err)
	case errors.Is(err, volfile.ErrExists):
		return JSONProblemf(ctx, http.StatusConflict, "File exists", "%s", err)
	case errors.Is(err, fs.ErrNotExist):
		return JSONProblemf(ctx, http.StatusNotFound, "No such file", "%s", err)
	default:
		return JSONProblemf(ctx, http.StatusInternalServerError, what, "%s", err)
	}
}

// GetInstanceFiles lists the entries of a directory of a volume.
func (a *DaemonAPI) GetInstanceFiles(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.GetInstanceFilesParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetInstanceFiles(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	dir, err := volumeDir(ctx, params.Dir)
	if ctx.Response().Committed {
		return err
	}
	head, err := a.volumeHead(ctx, namespace, kind, name)
	if noVolumeHead(ctx) {
		return err
	}
	l, err := volfile.List(head, dir)
	if err != nil {
		return volfileProblem(ctx, err, "List files")
	}
	items := make([]api.VolumeFile, len(l.Items))
	for i, f := range l.Items {
		items[i] = api.VolumeFile{Name: f.Name, Type: api.VolumeFileType(f.Type), Size: f.Size, ModifiedAt: f.ModifiedAt}
		if f.UploadedBy != "" {
			items[i].UploadedBy = &f.UploadedBy
		}
		if !f.UploadedAt.IsZero() {
			items[i].UploadedAt = &f.UploadedAt
		}
		if f.Type == volfile.TypeLink {
			items[i].Target = &f.Target
		}
	}
	return ctx.JSON(http.StatusOK, api.VolumeFileList{Kind: "VolumeFileList", Items: items, Truncated: l.Truncated})
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
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid file name", "%q: a name of an entry of the directory is expected", params.Name)
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostInstanceFile(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	dir, err := volumeDir(ctx, params.Dir)
	if ctx.Response().Committed {
		return err
	}
	// Said before a transfer is started, rather than at its end.
	head, err := a.volumeHead(ctx, namespace, kind, name)
	if noVolumeHead(ctx) {
		return err
	}
	replace := params.Replace != nil && *params.Replace
	// Asked again as the transfer ends, for a file created meanwhile.
	if exists, err := volfile.Exists(head, dir, params.Name); err != nil {
		return volfileProblem(ctx, err, "Stat file")
	} else if exists && !replace {
		return JSONProblemf(ctx, http.StatusConflict, "File exists", "%s exists: set replace to replace it", params.Name)
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid path", "%s", err)
	}
	return a.issueConsoleTicket(ctx, console.Target{Node: a.localhost, Path: p.String(), Kind: console.KindUpload, Dir: dir, File: params.Name, Replace: replace})
}

// PostInstanceFileDirectory makes a directory of a volume.
func (a *DaemonAPI) PostInstanceFileDirectory(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceFileDirectoryParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	if !volfile.IsName(params.Name) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid directory name", "%q: a name of an entry of the directory is expected", params.Name)
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostInstanceFileDirectory(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	dir, err := volumeDir(ctx, params.Dir)
	if ctx.Response().Committed {
		return err
	}
	head, err := a.volumeHead(ctx, namespace, kind, name)
	if noVolumeHead(ctx) {
		return err
	}
	if err := volfile.Mkdir(head, dir, params.Name); err != nil {
		return volfileProblem(ctx, err, "Make directory")
	}
	return ctx.NoContent(http.StatusCreated)
}

// DeleteInstanceFile removes an entry of a directory of a volume.
func (a *DaemonAPI) DeleteInstanceFile(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.DeleteInstanceFileParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	if !volfile.IsName(params.Name) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid file name", "%q: a name of an entry of the directory is expected", params.Name)
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.DeleteInstanceFile(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	dir, err := volumeDir(ctx, params.Dir)
	if ctx.Response().Committed {
		return err
	}
	head, err := a.volumeHead(ctx, namespace, kind, name)
	if noVolumeHead(ctx) {
		return err
	}
	if err := volfile.Remove(head, dir, params.Name); err != nil {
		return volfileProblem(ctx, err, "Remove file")
	}
	return ctx.NoContent(http.StatusNoContent)
}

// PostInstanceFileDownload issues the ticket a file of a volume is read with,
// through the console listener, as an upload sends one: a volume holds files
// of several GiB, which are no reason to have the daemon read them.
func (a *DaemonAPI) PostInstanceFileDownload(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceFileDownloadParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	if !volfile.IsName(params.Name) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid file name", "%q: a name of an entry of the directory is expected", params.Name)
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostInstanceFileDownload(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	dir, err := volumeDir(ctx, params.Dir)
	if ctx.Response().Committed {
		return err
	}
	head, err := a.volumeHead(ctx, namespace, kind, name)
	if noVolumeHead(ctx) {
		return err
	}
	if _, err := volfile.Stat(head, dir, params.Name); err != nil {
		if errors.Is(err, volfile.ErrNotFile) {
			return JSONProblemf(ctx, http.StatusConflict, "Not a file", "%s", err)
		}
		return volfileProblem(ctx, err, "Stat file")
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid path", "%s", err)
	}
	return a.issueConsoleTicket(ctx, console.Target{Node: a.localhost, Path: p.String(), Kind: console.KindDownload, Dir: dir, File: params.Name})
}
