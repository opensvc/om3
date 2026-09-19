package daemonapi

import (
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

// claimGrantsSeededFor is the node the grant tables were rebuilt for, and is
// empty while they have been rebuilt for nobody.
//
// The tables belong to the node speaking for the cluster, and that node
// changes: one that takes over has tables of its own, empty. Remembering who
// they were rebuilt for is what notices both the taking over and the giving
// up, without anything having to be watched.
var claimGrantsSeededFor atomic.Value

// seedClaimGrants rebuilds what this node has granted and not yet seen, from
// what its peers already hold, and does it once per node speaking for the
// cluster.
//
// A claim is answered from the configurations the cluster shares, completed
// by what this node has granted since: a write reaches that reading a moment
// after it is made. The grants live in memory, so a node that has just
// started, or has just taken the answering over, has none of them, and
// answers claims from a reading it knows to be behind.
//
// What was granted and written is on the node that wrote it, whose own
// reading of its own objects has no lag, so asking every node what it holds
// finds the writes this one has not seen. It is asked once, when the
// answering moves, because that is the only moment the tables are known to be
// behind.
func (a *DaemonAPI) seedClaimGrants(ctx echo.Context, speaker string) {
	if seededFor, _ := claimGrantsSeededFor.Load().(string); seededFor == speaker {
		return
	}
	claimGrantsSeededFor.Store(speaker)
	if speaker != a.localhost {
		// The tables of another node are not this one's to rebuild. Saying
		// who they are its is what makes taking the answering back rebuild
		// them.
		return
	}
	log := LogHandler(ctx, "seedClaimGrants")
	now := time.Now()
	pools, networks := 0, 0
	local := make(map[string]int64)
	localCharges := make(map[string]map[string]int64)
	for _, item := range getPoolVolumes(nil) {
		local[item.Path] = item.Size
		if item.Charges != nil {
			localCharges[item.Path] = *item.Charges
		}
	}
	localIP := make(map[string]bool)
	for _, ip := range GetClusterIPs() {
		localIP[ipam.Key(ip.Path, ip.RID)] = true
	}
	for _, nodename := range clusternode.Get() {
		if nodename == a.localhost {
			continue
		}
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			log.Tracef("claims: ask %s what it holds: %s", nodename, err)
			continue
		}
		pools += a.seedPoolGrants(ctx, c, local, localCharges, now)
		networks += a.seedNetworkGrants(ctx, c, localIP, now)
	}
	// Said whatever it found, because it is said once for as long as this
	// node answers the claims of the cluster, and what it found is how far
	// behind its reading was when it took them over.
	log.Infof("claims: %d volume and %d address grants rebuilt from the peers", pools, networks)
}

// seedPoolGrants records what a peer holds of a pool that this node has not
// seen yet, and answers how many.
func (a *DaemonAPI) seedPoolGrants(ctx echo.Context, c *client.T, local map[string]int64, localCharges map[string]map[string]int64, now time.Time) int {
	resp, err := c.GetPoolVolumesWithResponse(ctx.Request().Context(), &api.GetPoolVolumesParams{})
	if err != nil || resp.JSON200 == nil {
		return 0
	}
	var n int
	for _, item := range resp.JSON200.Items {
		p, err := naming.ParsePath(item.Path)
		if err != nil {
			continue
		}
		if item.Size > local[item.Path] {
			poolClaimGrants.Seed(p.Namespace, item.Pool, item.Path, item.Size, now)
			n++
		}
		if item.Charges == nil {
			continue
		}
		// What a volume takes of a pool that did not serve it moves with its
		// configuration, so it is behind here for the same reason, and not
		// always together with the size.
		for chargedPool, chargedSize := range *item.Charges {
			if chargedSize <= localCharges[item.Path][chargedPool] {
				continue
			}
			poolClaimGrants.Seed(p.Namespace, chargedPool, item.Path, chargedSize, now)
		}
	}
	return n
}

// seedNetworkGrants records the addresses a peer holds that this node has not
// seen yet, and answers how many.
func (a *DaemonAPI) seedNetworkGrants(ctx echo.Context, c *client.T, local map[string]bool, now time.Time) int {
	resp, err := c.GetNetworkIPWithResponse(ctx.Request().Context(), &api.GetNetworkIPParams{})
	if err != nil || resp.JSON200 == nil {
		return 0
	}
	var n int
	for _, item := range resp.JSON200.Items {
		p, err := naming.ParsePath(item.Path)
		if err != nil {
			continue
		}
		key := ipam.Key(p, item.RID)
		if local[key] {
			continue
		}
		networkClaimGrants.Seed(p.Namespace, item.Network.Name, key, now)
		n++
	}
	return n
}
