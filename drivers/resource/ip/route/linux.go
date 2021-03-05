// +build linux

package main

import (
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"opensvc.com/opensvc/core/status"
)

// Start the Resource
func (r Type) Start() error {
	return nil
}

// Stop the Resource
func (r Type) Stop() error {
	return nil
}

// Label returns a formatted short description of the Resource
func (r Type) Label() string {
	return fmt.Sprintf("%s via %s", r.Destination, r.Gateway)
}

// Status evaluates and display the Resource status and logs
func (r *Type) Status() status.Type {
	netns, err := ns.GetNS(r.Netns)
	if err != nil {
		r.Log().Error("failed to open netns %q: %v", r.Netns, err)
		return status.Down
	}
	defer netns.Close()

	if err := netns.Do(func(_ ns.NetNS) error {
		routes, errM := r.makeRoute()
		if errM != nil {
			return errM
		}
		errV := ip.ValidateExpectedRoute(routes)
		if errV != nil {
			return errV
		}
		return nil
	}); err != nil {
		r.Log().Error("%v", err)
		return status.Down
	}

	return status.Up
}

func (r *Type) makeRoute() ([]*types.Route, error) {
	var routes []*types.Route
	_, dst, err := net.ParseCIDR(r.Destination)
	if err != nil {
		return routes, err
	}
	gw := net.ParseIP(r.Gateway)
	routes = append(
		routes,
		&types.Route{Dst: *dst, GW: gw},
	)
	return routes, nil
}
