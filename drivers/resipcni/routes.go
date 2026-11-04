package resipcni

import (
	"encoding/json"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

// heldRoutesIn returns the destinations of the routes of the main table of
// the network namespace, the default ones as 0.0.0.0/0 and ::/0.
func heldRoutesIn(netns ns.NetNS) (map[string]bool, error) {
	held := make(map[string]bool)
	err := netns.Do(func(_ ns.NetNS) error {
		routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
		if err != nil {
			return err
		}
		for _, r := range routes {
			held[routeDst(r)] = true
		}
		return nil
	})
	return held, err
}

// routeDst is the destination of a route, as a cni configuration writes it.
func routeDst(r netlink.Route) string {
	if r.Dst != nil {
		return r.Dst.String()
	}
	if r.Family == netlink.FAMILY_V6 || (r.Gw != nil && r.Gw.To4() == nil) {
		return "::/0"
	}
	return "0.0.0.0/0"
}

// dropHeldRoutes removes from the ipam routes of the plugin configuration
// conf the ones whose destination the network namespace has a route to
// already, as the default route of the first address added to it.
//
// Each om network asks for a default route and a route to its network. The
// cni plugins of a version add a route to a destination held already, where
// the earlier ones skipped it: the default route of a container holding
// addresses of two networks was the one of the last added, by a gateway
// the replies to the other network can not leave by. The routes held are
// kept, the first address added, the lowest resource id, setting them, as
// ip.netns does and as the earlier plugins did.
func dropHeldRoutes(conf []byte, held map[string]bool) ([]byte, []string, error) {
	var m map[string]any
	if err := json.Unmarshal(conf, &m); err != nil {
		return nil, nil, err
	}
	ipam, ok := m["ipam"].(map[string]any)
	if !ok {
		return conf, nil, nil
	}
	routes, ok := ipam["routes"].([]any)
	if !ok {
		return conf, nil, nil
	}
	kept := make([]any, 0, len(routes))
	var dropped []string
	for _, route := range routes {
		if r, ok := route.(map[string]any); ok {
			if dst, ok := r["dst"].(string); ok {
				if _, ipNet, err := net.ParseCIDR(dst); err == nil && held[ipNet.String()] {
					dropped = append(dropped, ipNet.String())
					continue
				}
			}
		}
		kept = append(kept, route)
	}
	if len(dropped) == 0 {
		return conf, nil, nil
	}
	ipam["routes"] = kept
	b, err := json.Marshal(m)
	return b, dropped, err
}
