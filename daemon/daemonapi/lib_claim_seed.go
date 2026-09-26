package daemonapi

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/claim"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
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
	if speaker != a.localhost {
		// The tables of another node are not this one's to rebuild. Saying
		// who they are its is what makes taking the answering back rebuild
		// them.
		claimGrantsSeededFor.Store(speaker)
		return
	}
	log := LogHandler(ctx, "seedClaimGrants")
	now := time.Now()
	pools, networks, computes := 0, 0, 0
	asked := 0
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
	peers := 0
	for _, nodename := range clusternode.Get() {
		if nodename == a.localhost {
			continue
		}
		peers++
		// The node asks, not whoever asked it. What a peer holds of a pool
		// is read by the root grant, and a claim is answered to an
		// administrator of a namespace: rebuilding through their grants
		// reads nothing, and a table that could not be rebuilt is worse than
		// one rebuilt late.
		c, err := newPeerClient(nodename)
		if err != nil {
			log.Tracef("claims: ask %s what it holds: %s", nodename, err)
			continue
		}
		poolCount, err := a.seedPoolGrants(ctx, c, local, localCharges, now)
		if err != nil {
			log.Tracef("claims: ask %s what volumes it holds: %s", nodename, err)
			continue
		}
		networkCount, err := a.seedNetworkGrants(ctx, c, localIP, now)
		if err != nil {
			log.Tracef("claims: ask %s what addresses it holds: %s", nodename, err)
			continue
		}
		computeCount, err := a.seedComputeGrants(ctx, c, nodename, now)
		if err != nil {
			log.Tracef("claims: ask %s what compute its objects claim: %s", nodename, err)
			continue
		}
		asked++
		pools += poolCount
		networks += networkCount
		computes += computeCount
	}
	if asked < peers {
		// A rebuild that did not reach every peer is not one: what it missed
		// is what it exists to find. Leaving the tables unmarked is what
		// makes the next claim try again.
		log.Warnf("claims: %d of %d peers answered what they hold, so the grants are rebuilt on the next claim", asked, peers)
		return
	}
	claimGrantsSeededFor.Store(speaker)
	// Said whatever it found, because it is said once for as long as this
	// node answers the claims of the cluster, and what it found is how far
	// behind its reading was when it took them over.
	log.Infof("claims: %d volume, %d address and %d compute grants rebuilt from the peers", pools, networks, computes)
}

// seedPoolGrants records what a peer holds of a pool that this node has not
// seen yet, and answers how many.
func (a *DaemonAPI) seedPoolGrants(ctx echo.Context, c *client.T, local map[string]int64, localCharges map[string]map[string]int64, now time.Time) (int, error) {
	resp, err := c.GetPoolVolumesWithResponse(ctx.Request().Context(), &api.GetPoolVolumesParams{})
	if err != nil {
		return 0, err
	}
	if resp.JSON200 == nil {
		return 0, fmt.Errorf("unexpected status code %d", resp.StatusCode())
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
	return n, nil
}

// seedComputeGrants records what the objects of a peer claim of the compute
// that this node has not seen yet, and answers how many.
//
// A peer is asked about its own instances only: what it publishes of its own
// objects has no lag there, and what it knows of the others is the reading
// this node has too.
func (a *DaemonAPI) seedComputeGrants(ctx echo.Context, c *client.T, nodename string, now time.Time) (int, error) {
	resp, err := c.GetInstancesWithResponse(ctx.Request().Context(), &api.GetInstancesParams{Node: &nodename})
	if err != nil {
		return 0, err
	}
	if resp.JSON200 == nil {
		return 0, fmt.Errorf("unexpected status code %d", resp.StatusCode())
	}
	var n int
	for _, item := range resp.JSON200.Items {
		if item.Meta.Node != nodename {
			continue
		}
		cfg := item.Data.Config
		if cfg == nil || cfg.ActorConfig == nil || len(cfg.Claims) == 0 {
			continue
		}
		p, err := naming.ParsePath(item.Meta.Object)
		if err != nil {
			continue
		}
		local := configuredComputeClaims(p)
		for claimType, v := range cfg.Claims {
			if v == claim.Unbounded || v <= local[claimType] {
				continue
			}
			grants, ok := computeClaimGrants[claimType]
			if !ok {
				continue
			}
			grants.Seed(p.Namespace, claimType, p.String(), v, now)
			n++
		}
	}
	return n, nil
}

// seedNetworkGrants records the addresses a peer holds that this node has not
// seen yet, and answers how many.
func (a *DaemonAPI) seedNetworkGrants(ctx echo.Context, c *client.T, local map[string]bool, now time.Time) (int, error) {
	resp, err := c.GetNetworkIPWithResponse(ctx.Request().Context(), &api.GetNetworkIPParams{})
	if err != nil {
		return 0, err
	}
	if resp.JSON200 == nil {
		return 0, fmt.Errorf("unexpected status code %d", resp.StatusCode())
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
	return n, nil
}

// newPeerClient is a client to a peer, as this node rather than as whoever
// asked it something.
func newPeerClient(nodename string) (*client.T, error) {
	tk, err := daemonauth.CreateNodeToken()
	if err != nil {
		return nil, err
	}
	return client.New(
		client.WithURL(daemonsubsystem.PeerURL(nodename)),
		client.WithBearer(tk),
		client.WithCertificate(daemonenv.CertChainFile()),
	)
}
