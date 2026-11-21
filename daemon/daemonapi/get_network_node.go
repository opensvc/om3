package daemonapi

import (
	"net/http"
	"sort"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/netstatus"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/daemon/api"
)

// GetNetworkNodes returns what the last setup of each network made on each
// node, as the nodes published it: the root and inspector grants read it, as
// it exposes the addresses the nodes reach each other at.
func (a *DaemonAPI) GetNetworkNodes(ctx echo.Context, params api.GetNetworkNodesParams) error {
	if v, err := assertInspector(ctx); !v {
		return err
	}
	var nodeMap nodeselector.ResultMap
	if params.Node != nil {
		selector := a.parseNodename(*params.Node)
		var err error
		nodeMap, err = nodeselector.New(selector).ExpandMap()
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Internal Server Error", "expand node selection %s: %s", selector, err)
		}
	}
	items := make(api.NetworkNodeItems, 0)
	for _, e := range netstatus.StatusData.GetAll() {
		if params.Name != nil && *params.Name != e.Name {
			continue
		}
		if nodeMap != nil && !nodeMap.Has(e.Node) {
			continue
		}
		items = append(items, newNetworkNode(e))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Node < items[j].Node
	})
	return ctx.JSON(http.StatusOK, api.NetworkNodeList{Kind: "NetworkNodeList", Items: items})
}

func newNetworkNode(e netstatus.Element) api.NetworkNode {
	st := e.Value
	item := api.NetworkNode{
		Name:      e.Name,
		Node:      e.Node,
		Type:      st.Type,
		Network:   st.Network,
		AppliedAt: st.AppliedAt,
		Peers:     make([]api.NetworkPeer, 0, len(st.Peers)),
	}
	if st.Dev != "" {
		item.Dev = &st.Dev
	}
	if st.Subnet != "" {
		item.Subnet = &st.Subnet
	}
	if st.Skipped != "" {
		item.Skipped = &st.Skipped
	}
	if st.Error != "" {
		item.Error = &st.Error
	}
	for _, p := range st.Peers {
		peer := api.NetworkPeer{
			Nodename: p.Nodename,
			Subnet:   p.Subnet,
			Tunnel:   p.Tunnel,
		}
		if p.Local != nil {
			peer.Local = p.Local.String()
		}
		if p.Remote != nil {
			peer.Remote = p.Remote.String()
		}
		if p.Error != "" {
			peer.Error = &p.Error
		}
		if p.TunnelMode != "" {
			peer.TunnelMode = &p.TunnelMode
		}
		if p.Via != "" {
			via := api.NetworkPeerVia(p.Via)
			peer.Via = &via
		}
		item.Peers = append(item.Peers, peer)
	}
	if len(st.Ports) > 0 {
		ports := make([]api.NetworkPort, 0, len(st.Ports))
		for _, p := range st.Ports {
			port := api.NetworkPort{Name: p.Name, Dev: p.Dev}
			if p.Error != "" {
				port.Error = &p.Error
			}
			ports = append(ports, port)
		}
		item.Ports = &ports
	}
	return item
}
