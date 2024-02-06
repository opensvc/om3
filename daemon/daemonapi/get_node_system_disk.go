package daemonapi

import (
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/object"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodeSystemDisk(ctx echo.Context, nodename api.InPathNodeName) error {
	if a.localhost == nodename {
		return a.getLocalNodeSystemDisk(ctx)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.getPeerNodeSystemDisk(ctx, nodename)
	}
}

func (a *DaemonAPI) getPeerNodeSystemDisk(ctx echo.Context, nodename string) error {
	c, err := newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	if resp, err := c.GetNodeSystemDiskWithResponse(ctx.Request().Context(), nodename); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonAPI) getLocalNodeSystemDisk(ctx echo.Context) error {
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	l, err := n.LoadDisks()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Load disks cache", "%s", err)
	}
	items := make(api.DiskItems, len(l))
	for i := 0; i < len(l); i++ {
		regions := make([]api.Region, len(l[i].Regions))

		for j := 0; j < len(l[i].Regions); j++ {
			regions[j] = api.Region{
				ID:      l[i].Regions[j].ID,
				Group:   l[i].Regions[j].Group,
				Object:  l[i].Regions[j].Object,
				Devpath: l[i].Regions[j].DevPath,
				Size:    l[i].Regions[j].Size,
			}
		}

		items[i] = api.DiskItem{
			Kind: "DiskItem",
			Data: api.Disk{
				ID:      l[i].ID,
				Size:    l[i].Size,
				Vendor:  l[i].Vendor,
				Model:   l[i].Model,
				Type:    l[i].Type,
				Devpath: l[i].DevPath,
				Regions: regions,
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		}
	}

	return ctx.JSON(http.StatusOK, api.DiskList{Kind: "DiskList", Items: items})
}
