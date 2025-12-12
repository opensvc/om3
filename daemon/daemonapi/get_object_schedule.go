package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetObjectSchedule(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	path, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New path", "%s", err)
	}
	items := make(api.ScheduleItems, 0)
	for nodename := range instance.MonitorData.GetByPath(path) {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		} else if !clusternode.Has(nodename) {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
		}
		if resp, err := c.GetInstanceSchedule(ctx.Request().Context(), nodename, namespace, kind, name); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else {
			switch resp.StatusCode {
			case http.StatusOK:
				var more api.ScheduleList
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
	resp := api.ScheduleList{
		Kind:  "ScheduleList",
		Items: items,
	}
	return ctx.JSON(http.StatusOK, resp)
}
