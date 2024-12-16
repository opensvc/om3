package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetObjectResourceInfo(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	path, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New path", "%s", err)
	}
	items := make(api.ResourceInfoItems, 0)
	for nodename, _ := range instance.MonitorData.GetByPath(path) {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		} else if !clusternode.Has(nodename) {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
		}
		if resp, err := c.GetInstanceResourceInfo(ctx.Request().Context(), nodename, namespace, kind, name); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else {
			switch resp.StatusCode {
			case http.StatusOK:
				var more api.ResourceInfoList
				dec := json.NewDecoder(resp.Body)
				if err := dec.Decode(&more); err != nil {
					return JSONProblemf(ctx, http.StatusInternalServerError, "Decode proxy response body", "%s: %s", nodename, err)
				}
				items = append(items, more.Items...)
			default:
				ctx.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
			}
		}
	}
	resp := api.ResourceInfoList{
		Kind:  "ResourceInfoList",
		Items: items,
	}
	return ctx.JSON(http.StatusOK, resp)
}
