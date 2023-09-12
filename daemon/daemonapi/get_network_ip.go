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

func GetClusterIps() clusterip.L {
	cips := make(clusterip.L, 0)
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
			cip := clusterip.T{
				Path: instStatusData.Path,
				Node: instStatusData.Node,
				RID:  rid,
				IP:   net.ParseIP(ipaddr),
			}
			cips = append(cips, cip)
		}
	}
	return cips
}

// GetNetworks returns network status list.
func (a *DaemonApi) GetNetworkIp(ctx echo.Context, params api.GetNetworkIpParams) error {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Failed to allocate a new object.Node", fmt.Sprint(err))
	}
	cips := GetClusterIps()
	var networkStatusList network.StatusList
	if params.Name != nil {
		networkStatusList = network.ShowNetworksByName(n, *params.Name, cips)
	} else {
		networkStatusList = network.ShowNetworks(n, cips)
	}
	var l api.NetworkIpArray
	for _, networkStatus := range networkStatusList {
		for _, ip := range networkStatus.IPs {
			l = append(l, api.NetworkIp{
				Path: ip.Path.String(),
				Node: ip.Node,
				Rid:  ip.RID,
				Ip:   ip.IP.String(),
				Network: api.NetworkIpNetwork{
					Name:    networkStatus.Name,
					Type:    networkStatus.Type,
					Network: networkStatus.Network,
				},
			})
		}
	}
	return ctx.JSON(http.StatusOK, l)
}
