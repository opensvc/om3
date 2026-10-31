package daemonapi

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"syscall"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/daemon/api"
)

// uploadPrefix starts the name a file is written under until it is complete.
const uploadPrefix = ".om-upload-"

// isVolumeFileName says a name is the one of a file at the top of a volume: no
// path, and not what a listing leaves out.
func isVolumeFileName(s string) bool {
	return s != "" && !strings.HasPrefix(s, ".") && !strings.ContainsAny(s, "/\\\x00") && len(s) <= 255
}

// errVolumeFull is a file not fitting in the size of a volume.
var errVolumeFull = errors.New("volume full")

// room returns what a file of the name may take in a volume of the size: what
// the other files leave, the one of that name being replaced. A filesystem
// bounds what it holds, but not the directory of a pool of that type, whose
// volumes share the one of the node.
func room(head, name string, size int64) (int64, error) {
	l, err := listVolumeFiles(head)
	if err != nil {
		return 0, err
	}
	for _, f := range l {
		if f.Name != name {
			size -= f.Size
		}
	}
	return max(size, 0), nil
}

// writeVolumeFile writes r to the file of the name in head, limit bytes at
// most, none when negative.
//
// What a volume holds is its user's, a symbolic link to a file of the node
// included, and this runs as root: the root of the volume confines what is
// opened, and the file is written aside, for it to appear complete or not.
func writeVolumeFile(head, name string, r io.Reader, limit int64) (err error) {
	root, err := os.OpenRoot(head)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	tmp := uploadPrefix + name
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = root.Remove(tmp)
		}
	}()
	if limit >= 0 {
		r = io.LimitReader(r, limit+1)
	}
	n, err := io.Copy(f, r)
	if err == nil && limit >= 0 && n > limit {
		err = errVolumeFull
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return root.Rename(tmp, name)
}

// listVolumeFiles returns the files at the top of head.
func listVolumeFiles(head string) ([]api.VolumeFile, error) {
	root, err := os.OpenRoot(head)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	d, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = d.Close() }()
	entries, err := d.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	l := make([]api.VolumeFile, 0, len(entries))
	for _, e := range entries {
		if !e.Type().IsRegular() || !isVolumeFileName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		l = append(l, api.VolumeFile{Name: e.Name(), Size: info.Size(), ModifiedAt: info.ModTime()})
	}
	return l, nil
}

func removeVolumeFile(head, name string) error {
	root, err := os.OpenRoot(head)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return root.Remove(name)
}

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

// volumeSize returns the size a volume was served with, 0 when it says none.
func (a *DaemonAPI) volumeSize(namespace string, kind naming.Kind, name string) int64 {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return 0
	}
	cfg := instance.ConfigData.GetByPathAndNode(p, a.localhost)
	if cfg == nil || cfg.VolConfig == nil {
		return 0
	}
	return cfg.VolConfig.Size
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
	l, err := listVolumeFiles(head)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "List files", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.VolumeFileList{Kind: "VolumeFileList", Items: l})
}

// PutInstanceFile writes the body of the request to a file of a volume.
//
// The volume bounds what is written, its size being what the claims of the
// namespace count: the body is streamed there, and a client sending more than
// it holds is answered so.
func (a *DaemonAPI) PutInstanceFile(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PutInstanceFileParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	if !isVolumeFileName(params.Name) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid file name", "%q: a name at the top of the volume is expected", params.Name)
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		// The transfer lasts as long as its client takes.
		return a.proxyWith(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PutInstanceFileWithBody(ctx.Request().Context(), nodename, namespace, kind, name, &params, "application/octet-stream", ctx.Request().Body)
		}, nil, client.WithTimeout(0))
	}
	head, err := a.volumeHead(ctx, namespace, kind, name)
	if noVolumeHead(ctx) {
		return err
	}
	limit := int64(-1)
	if size := a.volumeSize(namespace, kind, name); size > 0 {
		if limit, err = room(head, params.Name, size); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "List files", "%s", err)
		}
	}
	err = writeVolumeFile(head, params.Name, ctx.Request().Body, limit)
	switch {
	case err == nil:
		return ctx.NoContent(http.StatusNoContent)
	case errors.Is(err, errVolumeFull), errors.Is(err, syscall.ENOSPC), errors.Is(err, syscall.EDQUOT):
		return JSONProblemf(ctx, http.StatusInsufficientStorage, "No space left", "%s is full: grow the volume", fmt.Sprintf("%s/vol/%s", namespace, name))
	default:
		return JSONProblemf(ctx, http.StatusInternalServerError, "Write file", "%s", err)
	}
}

// DeleteInstanceFile removes a file of a volume.
func (a *DaemonAPI) DeleteInstanceFile(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.DeleteInstanceFileParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	if !isVolumeFileName(params.Name) {
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
	err = removeVolumeFile(head, params.Name)
	switch {
	case err == nil:
		return ctx.NoContent(http.StatusNoContent)
	case errors.Is(err, fs.ErrNotExist):
		return JSONProblemf(ctx, http.StatusNotFound, "No such file", "%s", params.Name)
	default:
		return JSONProblemf(ctx, http.StatusInternalServerError, "Remove file", "%s", err)
	}
}
