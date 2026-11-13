package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
)

// GetNodeCollectorTagAttachments answers the tags attached to the node, as
// the collector answers them to the node. Root only, as the other reads of
// the node.
func (a *DaemonAPI) GetNodeCollectorTagAttachments(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetNodeCollectorTagAttachments(ctx.Request().Context(), nodename)
		})
	}
	cli, err := newCollectorTagClient(ctx)
	if cli == nil {
		return err
	}
	l, err := cli.Attached(naming.Path{})
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Collector tags attached", "%s", err)
	}
	r := make(api.CollectorTagAttachments, len(l))
	for i, e := range l {
		r[i] = api.CollectorTagAttachment{TagName: e.TagName, TagData: e.TagData, TagAttachData: e.TagAttachData}
	}
	return ctx.JSON(http.StatusOK, r)
}

// PostNodeCollectorTagAttachment attaches a tag to the node, on the
// collector: root only, as the other actions of the node.
func (a *DaemonAPI) PostNodeCollectorTagAttachment(ctx echo.Context, nodename api.InPathNodeName, name api.InPathCollectorTagName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	var body api.CollectorTagAttach
	if ctx.Request().ContentLength != 0 {
		if err := ctx.Bind(&body); err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
		}
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostNodeCollectorTagAttachment(ctx.Request().Context(), nodename, name, body)
		})
	}
	if name == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "the tag name must not be empty")
	}
	cli, err := newCollectorTagClient(ctx)
	if cli == nil {
		return err
	}
	LogHandler(ctx, "PostNodeCollectorTagAttachment").Infof("attach collector tag %s", name)
	info, err := cli.Attach(name, naming.Path{}, body.AttachData)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Attach collector tag", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.CollectorInfo{Info: info})
}

// DeleteNodeCollectorTagAttachment detaches a tag from the node, on the
// collector.
func (a *DaemonAPI) DeleteNodeCollectorTagAttachment(ctx echo.Context, nodename api.InPathNodeName, name api.InPathCollectorTagName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.DeleteNodeCollectorTagAttachment(ctx.Request().Context(), nodename, name)
		})
	}
	if name == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "the tag name must not be empty")
	}
	cli, err := newCollectorTagClient(ctx)
	if cli == nil {
		return err
	}
	LogHandler(ctx, "DeleteNodeCollectorTagAttachment").Infof("detach collector tag %s", name)
	info, err := cli.Detach(name, naming.Path{})
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Detach collector tag", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.CollectorInfo{Info: info})
}

// newCollectorTagClient returns a client of the tag routes of the collector,
// authenticated as this node.
func newCollectorTagClient(ctx echo.Context) (*collector.TagClient, error) {
	n, err := object.NewNode()
	if err != nil {
		return nil, JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	cli, err := n.CollectorServer()
	if err != nil {
		return nil, JSONProblemf(ctx, http.StatusInternalServerError, "Collector client", "%s", err)
	}
	return collector.NewTagClient(cli), nil
}
