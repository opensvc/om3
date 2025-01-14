//go:build linux

package resiproute

import (
	"context"
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"

	"github.com/opensvc/om3/core/actionresdeps"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
)

func (t *T) ActionResourceDeps() []actionresdeps.Dep {
	return []actionresdeps.Dep{
		{Action: "start", A: t.RID(), B: t.NetNS},
		{Action: "start", A: t.NetNS, B: t.RID()},
		{Action: "stop", A: t.NetNS, B: t.RID()},
	}
}

func (t *T) LinkTo() string {
	return t.NetNS
}

// Start the Resource
func (t *T) Start(ctx context.Context) error {
	netns, err := t.getNS(ctx)
	if err != nil {
		return err
	}
	defer netns.Close()
	if err := netns.Do(func(_ ns.NetNS) error {
		routes, errM := t.makeRoute()
		if errM != nil {
			return errM
		}
		if err := t.addRoutes(routes); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// Stop the Resource
func (t *T) Stop(ctx context.Context) error {
	netns, err := t.getNS(ctx)
	if err != nil {
		return err
	}
	defer netns.Close()
	if err := netns.Do(func(_ ns.NetNS) error {
		routes, errM := t.makeRoute()
		if errM != nil {
			return errM
		}
		if err := t.delRoutes(routes); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (t *T) getNS(ctx context.Context) (ns.NetNS, error) {
	if r := t.GetObjectDriver().ResourceByID(t.NetNS); r == nil {
		return nil, fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	} else if i, ok := r.(resource.NetNSPather); !ok {
		return nil, fmt.Errorf("resource %s pointed by the netns keyword does not expose a netns path", t.NetNS)
	} else if path, err := i.NetNSPath(ctx); err != nil {
		return nil, err
	} else {
		return ns.GetNS(path)
	}
}

// Status evaluates and display the Resource status and logs
func (t *T) Status(ctx context.Context) status.T {
	netns, err := t.getNS(ctx)
	if err != nil {
		return status.Down
	}
	defer netns.Close()

	if err := netns.Do(func(_ ns.NetNS) error {
		routes, errM := t.makeRoute()
		if errM != nil {
			return errM
		}
		errV := ip.ValidateExpectedRoute(routes)
		if errV != nil {
			return errV
		}
		return nil
	}); err != nil {
		//t.StatusLog().Error("%v", err)
		return status.Down
	}

	return status.Up
}

func delRoute(ipn *net.IPNet, gw net.IP, dev netlink.Link) error {
	return netlink.RouteDel(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       ipn,
		Gw:        gw,
	})
}

func (t *T) delRoutes(routes []*types.Route) error {
	dev, errl := t.dev()
	if errl != nil {
		return errl
	}
	for _, route := range routes {
		if route == nil {
			continue
		}
		if err := ip.ValidateExpectedRoute([]*types.Route{route}); err != nil {
			t.Log().Infof("route to %s dev %s already down", route.Dst.String(), dev.Attrs().Name)
			return nil
		}
		t.Log().Infof("del route to %s dev %s", route.Dst.String(), dev.Attrs().Name)
		return delRoute(&route.Dst, route.GW, dev)
	}
	return nil
}

func (t *T) addRoutes(routes []*types.Route) error {
	dev, errl := t.dev()
	if errl != nil {
		return errl
	}
	for _, route := range routes {
		if route == nil {
			continue
		}
		if err := ip.ValidateExpectedRoute([]*types.Route{route}); err == nil {
			t.Log().Infof("route to %s dev %s already up", route.Dst.String(), dev.Attrs().Name)
			return nil
		}
		t.Log().Infof("add route to %s dev %s", route.Dst, dev.Attrs().Name)
		return ip.AddRoute(&route.Dst, route.GW, dev)
	}
	return nil
}

func (t *T) makeRoute() ([]*types.Route, error) {
	var routes []*types.Route
	_, dst, err := net.ParseCIDR(t.To)
	if err != nil {
		return routes, err
	}
	gw := net.ParseIP(t.Gateway)
	routes = append(
		routes,
		&types.Route{Dst: *dst, GW: gw},
	)
	return routes, nil
}

func (t *T) dev() (netlink.Link, error) {
	if t.Dev != "" {
		return netlink.LinkByName(t.Dev)
	}
	return t.defaultDev()
}

func (t *T) defaultDev() (netlink.Link, error) {
	gw := net.ParseIP(t.Gateway)
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		ips, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			_, n, err := net.ParseCIDR(ip.String())
			if err != nil {
				continue
			}
			if n.Contains(gw) {
				return netlink.LinkByName(iface.Name)
			}
		}
	}
	return nil, fmt.Errorf("could not find a netdev to reach the gateway %s", t.Gateway)
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}
