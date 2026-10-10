package daemonapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

func (a *DaemonAPI) PostInstanceStateFile(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.postLocalObjectStateFile(ctx, namespace, kind, name)
	}
	relativePath := ctx.Request().Header.Get(api.HeaderRelativePath)
	lastModified := ctx.Request().Header.Get(api.HeaderLastModified)
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		addHeader := func(ctx context.Context, req *http.Request) error {
			req.Header.Add(api.HeaderRelativePath, relativePath)
			if lastModified != "" {
				req.Header.Add(api.HeaderLastModified, lastModified)
			}
			return nil
		}
		return c.PostInstanceStateFileWithBody(ctx.Request().Context(), nodename, namespace, kind, name, "application/octet-stream", ctx.Request().Body, addHeader)
	})

}

func (a *DaemonAPI) postLocalObjectStateFile(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request path", "%s", err)
	}
	if !p.Exists() {
		return JSONProblemf(ctx, http.StatusNotFound, "Object not found", "")
	}
	relPath := ctx.Request().Header.Get(api.HeaderRelativePath)
	if relPath == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request", "Header '%s' is required", api.HeaderRelativePath)
	}
	o, err := object.NewActor(p)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s", err)
	}
	headPath := o.(resource.ObjectDriver).VarDir()
	joinedPath := filepath.Join(headPath, relPath)
	joinedPath = filepath.Clean(joinedPath)
	if !filepath.HasPrefix(joinedPath, headPath) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Join file path", "The path '%s' is outside the allowed head path '%s'", joinedPath, headPath)
	}
	file, err := os.OpenFile(joinedPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(joinedPath), 0750); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Make state file directory", "%s", err)
		}
		file, err = os.OpenFile(joinedPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	}
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Write state file", "%s", err)
	}
	defer file.Close()
	if _, err := io.Copy(file, ctx.Request().Body); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Copy body to state file", "%s", err)
	}
	// A state file can say when something happened by its modification
	// time, as the last sync of a peer does, which writing it here would
	// make the time it was received.
	if s := ctx.Request().Header.Get(api.HeaderLastModified); s != "" {
		mtime, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad request", "Header '%s': %s", api.HeaderLastModified, err)
		}
		if err := os.Chtimes(joinedPath, mtime, mtime); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Set the state file modification time", "%s", err)
		}
	}
	a.Bus.Pub(&msgbus.InstanceStateFileUpdated{Path: p, Node: a.localhost, File: relPath},
		a.LabelLocalhost,
		pubsub.Label{"namespace", p.Namespace},
		pubsub.Label{"path", p.String()},
	)
	return ctx.NoContent(http.StatusNoContent)
}
