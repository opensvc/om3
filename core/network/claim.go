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

// ClaimHeld is how many addresses of a network a namespace already holds,
// cluster-wide.
//
// Addresses are counted, not reservations: a failover object holds the same
// address on every node it is configured on, and that is one address taken
// from the network, while the instances of a flex each hold one of their own.
//
// Two views are merged. The cluster reports the addresses its objects publish
// in their status, which is the only way to see the other nodes, but a status
// is published after the fact: an address reserved a moment ago is not in it
// yet. The local allocator has no such lag, and covers the case the cluster
// view misses most often, several resources of one instance allocating one
// after the other. An address in both is one address.
func ClaimHeld(ctx context.Context, i *ipam.T, namespace, networkName string) (int, error) {
	held := make(map[string]bool)
	reservations, err := i.Reservations()
	if err != nil {
		return 0, err
	}
	for _, reservation := range reservations {
		p, ok := ipam.PathOfKey(reservation.Key)
		if !ok || p.Namespace != namespace || reservation.IP == nil {
			continue
		}
		held[reservation.IP.String()] = true
	}
	c, err := client.New()
	if err != nil {
		return 0, err
	}
	resp, err := c.GetNetworkIPWithResponse(ctx, &api.GetNetworkIPParams{Name: &networkName})
	if err != nil {
		return 0, err
	}
	if resp.JSON200 == nil {
		return 0, fmt.Errorf("read the %s network addresses: unexpected status code %d", networkName, resp.StatusCode())
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
	}
	return len(held), nil
}

// ClaimFits says whether a namespace may take one more address of a network,
// and why not when it may not.
//
// A namespace with no claim on the network is not capped on it, and is
// answered from the local configuration alone, which is what most allocations
// are.
//
// What the namespace already holds has to be counted from the addresses the
// whole cluster holds, so that one is read through the daemon. Failing to
// reach it leaves the claim unchecked rather than refused, for the same reason
// a pool claim is: a cap is something the cluster brokers, and where there is
// no daemon to ask there is nothing brokering.
func ClaimFits(ctx context.Context, i *ipam.T, namespace string) (bool, string, error) {
	limit, capped, err := ClaimLimit(namespace, i.Name)
	if err != nil {
		return true, "", nil
	}
	if !capped {
		// The common case, and it asked nothing of the daemon.
		return true, "", nil
	}
	held, err := ClaimHeld(ctx, i, namespace, i.Name)
	if err != nil {
		return true, "", nil
	}
	if held+1 <= limit {
		return true, "", nil
	}
	return false, fmt.Sprintf("the %s namespace may hold %d address(es) of it and already holds %d",
		namespace, limit, held), nil
}
