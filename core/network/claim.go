package network

import (
	"context"
	"fmt"
	"strconv"

	"github.com/opensvc/om3/v3/core/claim"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

// ClaimLimit is the most addresses a namespace may hold in a network, and
// whether it is capped on it at all.
func ClaimLimit(namespace, networkName string) (int, bool, error) {
	limit, capped, err := claim.Limit(namespace, "network", networkName)
	if err != nil || !capped {
		return 0, false, err
	}
	n, err := strconv.Atoi(limit)
	if err != nil {
		return 0, false, fmt.Errorf("%s claim on the %s network: %s is not a number of addresses", namespace, networkName, limit)
	}
	return n, true, nil
}

// ClaimHeldByKey is how many addresses of a network a namespace holds, and
// the reservations they are held for.
//
// Addresses are counted, not reservations: a failover object holds the same
// address on every node it is configured on, and that is one address taken
// from the network, while the instances of a flex each hold one of their own.
// The reservations are answered alongside so that a claim granted a moment
// ago and not yet reported can be told apart from one the cluster has caught
// up with.
//
// Two views are merged. The cluster reports the addresses its objects publish
// in their status, which is the only way to see the other nodes, but a status
// is published after the fact: an address reserved a moment ago is not in it
// yet. The allocator of this node has no such lag, and this is read where
// every claim is answered, so what it adds is this node's own reservations,
// including the ones made before there was anything brokering. An address in
// both is one address.
func ClaimHeldByKey(ctx context.Context, networkName, namespace string) (int, map[string]bool, error) {
	held := make(map[string]bool)
	seen := make(map[string]bool)
	if i := localAllocator(networkName); i != nil {
		reservations, err := i.Reservations()
		if err != nil {
			return 0, nil, err
		}
		for _, reservation := range reservations {
			p, ok := ipam.PathOfKey(reservation.Key)
			if !ok || p.Namespace != namespace || reservation.IP == nil {
				continue
			}
			held[reservation.IP.String()] = true
			seen[reservation.Key] = true
		}
	}
	c, err := client.New()
	if err != nil {
		return 0, nil, err
	}
	resp, err := c.GetNetworkIPWithResponse(ctx, &api.GetNetworkIPParams{Name: &networkName})
	if err != nil {
		return 0, nil, err
	}
	if resp.JSON200 == nil {
		return 0, nil, fmt.Errorf("read the %s network addresses: unexpected status code %d", networkName, resp.StatusCode())
	}
	for _, item := range resp.JSON200.Items {
		p, err := naming.ParsePath(item.Path)
		if err != nil {
			continue
		}
		if p.Namespace != namespace {
			continue
		}
		held[item.IP] = true
		seen[ipam.Key(p, item.RID)] = true
	}
	return len(held), seen, nil
}

// localAllocator is the allocator of this node in a network, and is nil where
// the network is not one om allocates addresses in, or is not known here.
func localAllocator(networkName string) *ipam.T {
	nw, _, err := Lookup(networkName)
	if err != nil || nw == nil {
		return nil
	}
	i, err := NewAllocator(nw, hostname.Hostname())
	if err != nil {
		return nil
	}
	return i
}

// ClaimFits says whether a namespace may take one more address of a network
// for a resource of an object, and why not when it may not.
//
// A namespace with no claim on the network is not capped on it, and is
// answered from the local configuration alone, which is what most allocations
// are.
//
// A capped one is brokered. The claim is asked of the daemon, which hands the
// question to the node speaking for the cluster: what the namespace holds is
// read from what the objects of the cluster publish, an object publishes its
// address after it has taken it, and two claims answered from the same
// reading both fit where together they do not. One node answering them,
// counting what it has granted since, is what makes them fit one at a time.
//
// Failing to reach the daemon leaves the claim unchecked rather than refused,
// for the same reason a pool claim is: a cap is something the cluster
// brokers, and where there is no daemon to ask there is nothing brokering.
func ClaimFits(ctx context.Context, networkName, namespace string, p naming.Path, rid string) (bool, string, error) {
	if _, capped, err := ClaimLimit(namespace, networkName); err != nil || !capped {
		// The common case, and it asked nothing of the daemon.
		return true, "", nil
	}
	c, err := client.New()
	if err != nil {
		return true, "", nil
	}
	resp, err := c.PostNetworkClaimWithResponse(ctx, api.PostNetworkClaim{
		Namespace: namespace,
		Network:   networkName,
		Path:      p.String(),
		RID:       rid,
	})
	if err != nil || resp.JSON200 == nil {
		return true, "", nil
	}
	if resp.JSON200.Granted {
		return true, "", nil
	}
	var why string
	if resp.JSON200.Reason != nil {
		why = *resp.JSON200.Reason
	}
	return false, why, nil
}
