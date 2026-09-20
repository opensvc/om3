package daemonapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/daemon/api"
)

// claimGrantTTL is how long a claim answered yes is counted while the
// configuration claiming it has not come back around.
//
// It has to outlast the heartbeat that carries a configuration write to the
// peers, and the reading of it on the node that answered. What it costs when
// it is too long is a grant nobody claimed rationing the namespace until it
// expires, which only happens when a write fails after the claim was granted.
const claimGrantTTL = 30 * time.Second

// poolClaimGrants is what this node has let namespaces take of the pools and
// has not seen claimed yet. It is consulted only on the node speaking for the
// cluster, which is where every claim is answered.
var poolClaimGrants = pool.NewGrants(claimGrantTTL)

// PostPoolClaim answers whether a namespace may have an object hold a size of
// a pool, and counts the answer until the configuration saying so is seen.
//
// Every claim of the cluster is answered here, on one node, so that two of
// them asked at once are weighed against each other: what a namespace holds
// is read from the configurations the cluster shares, which a write reaches a
// moment after it is made, and claims answered from that reading alone all
// fit where together they do not.
//
// The node answering is the one speaking for the cluster, the first alive
// node of the cluster nodes, which every node agrees on and which already
// speaks for the cluster elsewhere. A node that is not it hands the question
// over. When the cluster has no speaker, which is a node that has not yet
// found its peers, the question is answered here: a claim nobody can broker
// is one the node weighs alone, as it did before there was a broker.
func (a *DaemonAPI) PostPoolClaim(ctx echo.Context) error {
	var payload api.PostPoolClaim
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid Body", err.Error())
	}
	if payload.Namespace == "" || payload.Pool == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid Body", "the namespace and the pool a claim is about are both needed")
	}
	if v, err := assertAdmin(ctx, payload.Namespace); !v {
		return err
	}
	speaker := speakerNode()
	a.seedClaimGrants(ctx, speaker)
	if speaker != "" && speaker != a.localhost {
		return a.proxy(ctx, speaker, func(c *client.T) (*http.Response, error) {
			return c.PostPoolClaim(ctx.Request().Context(), payload)
		})
	}
	var path string
	if payload.Path != nil {
		path = *payload.Path
	}
	limit, capped, err := pool.ClaimLimit(payload.Namespace, payload.Pool)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Claim limit", "%s", err)
	}
	if !capped {
		return ctx.JSON(http.StatusOK, api.PoolClaim{Granted: true})
	}
	c, err := client.New()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s", err)
	}
	held, err := pool.ClaimHeldByPath(ctx.Request().Context(), c, payload.Namespace, payload.Pool)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Pool volumes", "%s", err)
	}
	now := time.Now()
	var granted bool
	var why string
	if payload.Probe != nil && *payload.Probe {
		granted, why = poolClaimGrants.Probe(payload.Namespace, payload.Pool, path, payload.Size, limit, held, now)
	} else {
		granted, why = poolClaimGrants.Fits(payload.Namespace, payload.Pool, path, payload.Size, limit, held, now)
	}
	if !granted {
		return ctx.JSON(http.StatusOK, api.PoolClaim{Granted: false, Reason: &why})
	}
	if payload.Probe != nil && *payload.Probe {
		return ctx.JSON(http.StatusOK, api.PoolClaim{Granted: true})
	}
	expiresAt := poolClaimGrants.ExpiresAt(now)
	return ctx.JSON(http.StatusOK, api.PoolClaim{Granted: true, ExpiresAt: &expiresAt})
}

// speakerNode is the node speaking for the cluster, and is empty while no
// node does.
func speakerNode() string {
	for _, nodename := range clusternode.Get() {
		if data := node.StatusData.GetByNode(nodename); data != nil && data.IsLeader {
			return nodename
		}
	}
	return ""
}
