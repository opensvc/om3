package main

import (
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"opensvc.com/opensvc/core/status"
)

func (r R) Start() error {
	return nil
}

func (r R) Stop() error {
	return nil
}

func (r R) Label() string {
	return fmt.Sprintf("%s via %s", r.Destination, r.Gateway)
}

func (r R) Status() status.Type {
	netns, err := ns.GetNS(r.Netns)
	if err != nil {
		r.Log.Error("failed to open netns %q: %v", r.Netns, err)
		return status.Down
	}
	defer netns.Close()

	if err := netns.Do(func(_ ns.NetNS) error {
		var routes = r.MakeRoute()
		errV := ip.ValidateExpectedRoute(routes)
		if errV != nil {
			return errV
		}
		return nil
	}); err != nil {
		r.Log.Error("%v", err)
		return status.Down
	}

	return status.Up
}

func (r R) MakeRoute() []*types.Route {
	var routes []*types.Route
	_, dst, err := net.ParseCIDR(r.Destination)
	if err != nil {
		panic(err)
	}
	gw := net.ParseIP(r.Gateway)
	routes = append(
		routes,
		&types.Route{Dst: *dst, GW: gw},
	)
	return routes
}
