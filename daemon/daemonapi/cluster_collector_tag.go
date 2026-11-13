package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/daemon/api"
)

// GetClusterCollectorTags answers the tags of the collector, as it answers
// them to the node serving the request. The tags belong to no namespace: a
// guest of all of them.
func (a *DaemonAPI) GetClusterCollectorTags(ctx echo.Context) error {
	if v, err := assertGuest(ctx, ""); !v {
		return err
	}
	cli, err := newCollectorTagClient(ctx)
	if cli == nil {
		return err
	}
	l, err := cli.Names()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Collector tags", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.CollectorTagNames(nonNilStrings(l)))
}

// PostClusterCollectorTag creates a tag on the collector, as the node serving
// the request. The tag belongs to no namespace: an admin of all of them.
func (a *DaemonAPI) PostClusterCollectorTag(ctx echo.Context) error {
	if v, err := assertAdmin(ctx, ""); !v {
		return err
	}
	var body api.CollectorTagCreate
	if err := ctx.Bind(&body); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	if body.Name == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "the tag name must not be empty")
	}
	cli, err := newCollectorTagClient(ctx)
	if cli == nil {
		return err
	}
	LogHandler(ctx, "PostClusterCollectorTag").Infof("create collector tag %s", body.Name)
	info, err := cli.Create(body.Name, body.Data, body.Exclude)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Create collector tag", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.CollectorInfo{Info: info})
}
