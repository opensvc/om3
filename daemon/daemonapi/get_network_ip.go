package daemonapi

import (
	"fmt"
	"net"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusterip"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/network"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func GetClusterIPs() clusterip.L {
	clusterIPs := make(clusterip.L, 0)
	for _, instStatusData := range instance.StatusData.GetAll() {
		for rid, resStatus := range instStatusData.Value.Resources {
			i, ok := resStatus.Info["ipaddr"]
			if !ok {
				continue
			}
			ipaddr, ok := i.(string)
			if !ok {
				continue
			}
			clusterIP := clusterip.T{
				Path: instStatusData.Path,
				Node: instStatusData.Node,
				RID:  rid,
				IP:   net.ParseIP(ipaddr),
			}
			clusterIPs = append(clusterIPs, clusterIP)
		}
	}
	return clusterIPs
}

// GetNetworkIP returns network status list.
func (a *DaemonAPI) GetNetworkIP(ctx echo.Context, params api.GetNetworkIPParams) error {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Failed to allocate a new object.Node", fmt.Sprint(err))
	}
	clusterIPs := GetClusterIPs()
	var networkStatusList network.StatusList
	if params.Name != nil {
		networkStatusList = network.ShowNetworksByName(n, *params.Name, clusterIPs)
	} else {
		networkStatusList = network.ShowNetworks(n, clusterIPs)
	}
	var l api.NetworkIPItems
	for _, networkStatus := range networkStatusList {
		for _, ip := range networkStatus.IPs {
			l = append(l, api.NetworkIP{
				Path: ip.Path.String(),
				Node: ip.Node,
				RID:  ip.RID,
				IP:   ip.IP.String(),
				Network: api.NetworkIPNetwork{
					Name:    networkStatus.Name,
					Type:    networkStatus.Type,
					Network: networkStatus.Network,
				},
			})
		}
	}
	return ctx.JSON(http.StatusOK, api.NetworkIPList{Kind: "NetworkIPList", Items: l})
}
