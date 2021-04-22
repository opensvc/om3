// +build linux

package resiproute

import (
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"opensvc.com/opensvc/core/status"
)

// Start the Resource
func (t T) Start() error {
	return nil
}

// Stop the Resource
func (t T) Stop() error {
	return nil
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return fmt.Sprintf("%s via %s", t.Destination, t.Gateway)
}

// Status evaluates and display the Resource status and logs
func (t *T) Status() status.T {
	netns, err := ns.GetNS(t.Netns)
	if err != nil {
		t.StatusLog().Error("failed to open netns %q: %v", t.Netns, err)
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
		t.StatusLog().Error("%v", err)
		return status.Down
	}

	return status.Up
}

func (t *T) makeRoute() ([]*types.Route, error) {
	var routes []*types.Route
	_, dst, err := net.ParseCIDR(t.Destination)
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
