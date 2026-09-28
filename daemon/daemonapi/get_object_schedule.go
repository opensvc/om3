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
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
	}
	configs := instance.ConfigData.GetByPath(path)
	if len(configs) == 0 {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "object not found: %s", path)
	}
	items := make(api.ScheduleItems, 0)
	for nodename := range configs {
		if !clusternode.Has(nodename) {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
		}
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		resp, err := c.GetInstanceSchedule(ctx.Request().Context(), nodename, namespace, kind, name)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		}
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound {
				resp.Body.Close()
				continue
			}
			defer resp.Body.Close()
			return ctx.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
		}
		var more api.ScheduleList
		if err := func() error {
			defer resp.Body.Close()
			return json.NewDecoder(resp.Body).Decode(&more)
		}(); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Decode proxy response body", "%s: %s", nodename, err)
		}
		items = append(items, more.Items...)
	}
	resp := api.ScheduleList{
		Kind:  "ScheduleList",
		Items: items,
	}
	return ctx.JSON(http.StatusOK, resp)
}
