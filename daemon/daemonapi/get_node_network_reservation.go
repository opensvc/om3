package daemonapi

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
)

// GetNodeNetworkReservations returns the addresses a node reserved in its
// networks.
//
// It reads the reservation store of the node, where an address is the moment
// it is drawn, while the status of the object holding it says so only once
// the action drawing it is over. A node drawing an address of a network the
// cluster shares reads every node this way, under the lock of the network,
// and so sees what the node holding the lock before it drew.
func (a *DaemonAPI) GetNodeNetworkReservations(ctx echo.Context, nodename api.InPathNodeName, params api.GetNodeNetworkReservationsParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if nodename != a.localhost {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetNodeNetworkReservations(ctx.Request().Context(), nodename, &params)
		})
	}
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Node", "%s", err)
	}
	items := make([]api.NetworkReservation, 0)
	for _, nw := range network.Networks(n) {
		if params.Name != nil && *params.Name != nw.Name() {
			continue
		}
		i, err := network.NewAllocator(nw, a.localhost)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Allocator", "%s", err)
		}
		if i == nil {
			continue
		}
		reservations, err := i.Reservations()
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Reservations", "network %s: %s", nw.Name(), err)
		}
		for _, r := range reservations {
			p, ok := ipam.PathOfKey(r.Key)
			if !ok {
				continue
			}
			_, rid, _ := strings.Cut(r.Key, "!")
			items = append(items, api.NetworkReservation{
				Network: nw.Name(),
				IP:      r.IP.String(),
				Path:    p.String(),
				RID:     rid,
			})
		}
	}
	return ctx.JSON(http.StatusOK, api.NetworkReservationList{Kind: "NetworkReservationList", Items: items})
}
