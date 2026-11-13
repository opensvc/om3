package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
)

// PostNodeActionChecks runs the checks of the node, pushes their results to
// the collector, and answers them: also when the push failed, which the
// answer then says, as "om node checks" lists them and says so.
func (a *DaemonAPI) PostNodeActionChecks(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostNodeActionChecks(ctx.Request().Context(), nodename)
		})
	}
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	rs, err := n.Checks(ctx.Request().Context())
	return ctx.JSON(http.StatusOK, a.checkList(rs, err))
}

// checkList returns the results as the api answers them.
func (a *DaemonAPI) checkList(rs check.ResultSet, pushErr error) api.CheckList {
	r := api.CheckList{
		Kind:  api.CheckListKindCheckList,
		Items: make(api.CheckItems, len(rs.Data)),
	}
	for i, e := range rs.Data {
		r.Items[i] = api.CheckItem{
			Kind: "CheckItem",
			Meta: api.NodeMeta{Node: a.localhost},
			Data: api.CheckResult{
				Type:     e.DriverGroup,
				Driver:   e.DriverName,
				Path:     e.Path,
				Instance: e.Instance,
				Unit:     e.Unit,
				Value:    e.Value,
			},
		}
	}
	if pushErr != nil {
		s := pushErr.Error()
		r.PushError = &s
	}
	return r
}
