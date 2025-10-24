package daemonapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/daemon/api"
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
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		addHeader := func(ctx context.Context, req *http.Request) error {
			req.Header.Add(api.HeaderRelativePath, relativePath)
			return nil
		}
		return c.PostInstanceStateFileWithBody(ctx.Request().Context(), nodename, namespace, kind, name, "application/octet-stream", ctx.Request().Body, addHeader)
	})

}

func (a *DaemonAPI) postLocalObjectStateFile(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request path", fmt.Sprint(err))
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
	return ctx.NoContent(http.StatusNoContent)
}
