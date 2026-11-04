package resipcni

import (
	"context"
	"encoding/json"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/drivers/resip"
	"github.com/opensvc/om3/v3/util/hostname"
)

// heldRoutesIn returns the destinations of the routes of the main table of
// the network namespace, the default ones as 0.0.0.0/0 and ::/0. The tables
// of the addresses, see resip.SourceRoute, are not the namespace's routing.
func heldRoutesIn(netns ns.NetNS) (map[string]bool, error) {
	held := make(map[string]bool)
	err := netns.Do(func(_ ns.NetNS) error {
		routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{Table: unix.RT_TABLE_MAIN}, netlink.RT_FILTER_TABLE)
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

// sourceRoute is the policy routing of the address of the resource, on the
// card dev of the namespace: the address answers by the gateway of its
// network. A network om does not own, a cni configuration written by hand,
// has none: its plugin keeps the routing it was configured with.
func (t *T) sourceRoute(netns ns.NetNS, dev string) (resip.SourceRoute, bool) {
	nw, _, err := network.Lookup(t.Network)
	if err != nil || nw == nil || dev == "" {
		return resip.SourceRoute{}, false
	}
	addr, err := t.addrOnDev(netns, dev)
	if err != nil || addr == nil {
		return resip.SourceRoute{}, false
	}
	r := resip.SourceRoute{
		Addr: addr,
		Dev:  dev,
	}
	if segment, err := nw.IPNet(); err == nil {
		r.Network = segment
	}
	if i, err := network.NewAllocator(nw, hostname.Hostname()); err == nil && i != nil {
		r.Gateway = i.Gateway
	}
	return r, true
}

// startSourceRoute sets the policy routing of the address, once it is up.
func (t *T) startSourceRoute(ctx context.Context) error {
	netConf, err := t.netConf()
	if err != nil {
		return err
	}
	netns, err := t.getNS(ctx)
	if err != nil || netns == nil {
		return err
	}
	dev, err := t.currentGuestDev(netConf.IPAM.Subnet, netns)
	if err != nil {
		return err
	}
	r, ok := t.sourceRoute(netns, dev)
	if !ok {
		return nil
	}
	return r.Apply(netns, t.Log())
}

// addrOnDev returns the address of the network of the resource held by the
// card dev of the namespace, with its prefix: the card the resource holds,
// whatever its name, the nsdev one included.
func (t *T) addrOnDev(netns ns.NetNS, dev string) (*net.IPNet, error) {
	netConf, err := t.netConf()
	if err != nil {
		return nil, err
	}
	_, ref, err := net.ParseCIDR(netConf.IPAM.Subnet)
	if err != nil {
		return nil, err
	}
	var found *net.IPNet
	err = netns.Do(func(_ ns.NetNS) error {
		iface, err := net.InterfaceByName(dev)
		if err != nil {
			return err
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return err
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ref.Contains(ipNet.IP) {
				found = ipNet
				return nil
			}
		}
		return nil
	})
	return found, err
}
