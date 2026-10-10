package daemonapi

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/node"
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
		if until, ok := maintenanceEndsAt(a.localhost, nodename); ok {
			// Stopped cleanly, as for a restart, which is soon over: the
			// draw may wait for it rather than go without what it holds.
			// Its heartbeats may still be counted beating, but its daemon,
			// stopping or stopped, has nothing to answer.
			ctx.Response().Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(time.Until(until).Seconds()))))
			return JSONProblemf(ctx, http.StatusServiceUnavailable, "In maintenance", "the daemon of %s is stopped, in maintenance until %s at most", nodename, until.Format(time.RFC3339))
		}
		if !hearsPeer(a.localhost, nodename) {
			// Not alive, for what a draw reads of it: what it holds is in
			// the status the cluster kept of it, and in the peer records.
			return JSONProblemf(ctx, http.StatusNotFound, "Not alive", "the daemon of %s does not answer: no heartbeat is received from it", nodename)
		}
		return a.proxyOr(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetNodeNetworkReservations(ctx.Request().Context(), nodename, &params)
		}, func(err error) error {
			if mon := node.MonitorData.GetByNode(nodename); mon == nil || mon.State != node.MonitorStateRejoin {
				return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
			}
			// Its daemon just started: it beats before it listens, and
			// answers in a moment.
			ctx.Response().Header().Set("Retry-After", "1")
			return JSONProblemf(ctx, http.StatusServiceUnavailable, "Starting", "the daemon of %s is starting: %s", nodename, err)
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
