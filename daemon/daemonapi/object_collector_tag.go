package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

// GetObjectCollectorTagAttachments answers the tags attached to the object,
// as the collector answers them to the node serving the request: a read,
// guest.
func (a *DaemonAPI) GetObjectCollectorTagAttachments(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	p, err := clusterObjectPath(ctx, namespace, kind, name)
	if p.IsZero() {
		return err
	}
	cli, err := newCollectorTagClient(ctx)
	if cli == nil {
		return err
	}
	l, err := cli.Attached(p)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Collector tags attached", "%s", err)
	}
	r := make(api.CollectorTagAttachments, len(l))
	for i, e := range l {
		r[i] = api.CollectorTagAttachment{TagName: e.TagName, TagData: e.TagData, TagAttachData: e.TagAttachData}
	}
	return ctx.JSON(http.StatusOK, r)
}

// PostObjectCollectorTagAttachment attaches a tag to the object, on the
// collector: admin of the namespace.
func (a *DaemonAPI) PostObjectCollectorTagAttachment(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, tag api.InPathCollectorTagName) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	var body api.CollectorTagAttach
	if ctx.Request().ContentLength != 0 {
		if err := ctx.Bind(&body); err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
		}
	}
	p, err := clusterObjectPath(ctx, namespace, kind, name)
	if p.IsZero() {
		return err
	}
	if tag == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "the tag name must not be empty")
	}
	cli, err := newCollectorTagClient(ctx)
	if cli == nil {
		return err
	}
	naming.LogWithPath(LogHandler(ctx, "PostObjectCollectorTagAttachment"), p).Infof("attach collector tag %s", tag)
	info, err := cli.Attach(tag, p, body.AttachData)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Attach collector tag", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.CollectorInfo{Info: info})
}

// DeleteObjectCollectorTagAttachment detaches a tag from the object, on the
// collector: admin of the namespace.
func (a *DaemonAPI) DeleteObjectCollectorTagAttachment(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, tag api.InPathCollectorTagName) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	p, err := clusterObjectPath(ctx, namespace, kind, name)
	if p.IsZero() {
		return err
	}
	if tag == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "the tag name must not be empty")
	}
	cli, err := newCollectorTagClient(ctx)
	if cli == nil {
		return err
	}
	naming.LogWithPath(LogHandler(ctx, "DeleteObjectCollectorTagAttachment"), p).Infof("detach collector tag %s", tag)
	info, err := cli.Detach(tag, p)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Detach collector tag", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.CollectorInfo{Info: info})
}
