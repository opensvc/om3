package daemonapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/daemon/api"
)

// networkClaimGrants is what this node has let namespaces take of the
// networks and has not seen reserved yet. It is consulted only on the node
// speaking for the cluster, which is where every claim is answered.
var networkClaimGrants = network.NewGrants(claimGrantTTL)

// PostNetworkClaim answers whether a namespace may take one more address of a
// network, and counts the answer until the cluster reports the address.
//
// Every claim of the cluster is answered here, on the node speaking for it,
// for the reason a pool claim is: what a namespace holds is read from what
// the objects publish, an object publishes its address after it has taken it,
// and claims answered from that reading alone all fit where together they do
// not.
func (a *DaemonAPI) PostNetworkClaim(ctx echo.Context) error {
	var payload api.PostNetworkClaim
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid Body", err.Error())
	}
	if payload.Namespace == "" || payload.Network == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid Body", "the namespace and the network a claim is about are both needed")
	}
	if v, err := assertAdmin(ctx, payload.Namespace); !v {
		return err
	}
	if speaker := speakerNode(); speaker != "" && speaker != a.localhost {
		return a.proxy(ctx, speaker, func(c *client.T) (*http.Response, error) {
			return c.PostNetworkClaim(ctx.Request().Context(), payload)
		})
	}
	p, err := naming.ParsePath(payload.Path)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid Body", "the object the address is for: %s", err)
	}
	limit, capped, err := network.ClaimLimit(payload.Namespace, payload.Network)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Claim limit", "%s", err)
	}
	if !capped {
		return ctx.JSON(http.StatusOK, api.NetworkClaim{Granted: true})
	}
	held, seen, err := network.ClaimHeldByKey(ctx.Request().Context(), payload.Network, payload.Namespace)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Network addresses", "%s", err)
	}
	now := time.Now()
	granted, why := networkClaimGrants.Fits(payload.Namespace, payload.Network, ipam.Key(p, payload.RID), limit, held, seen, now)
	if !granted {
		return ctx.JSON(http.StatusOK, api.NetworkClaim{Granted: false, Reason: &why})
	}
	expiresAt := networkClaimGrants.ExpiresAt(now)
	return ctx.JSON(http.StatusOK, api.NetworkClaim{Granted: true, ExpiresAt: &expiresAt})
}
