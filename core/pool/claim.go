package pool

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/claim"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

// ClaimLimit is the most a namespace may claim of a pool, and whether it is
// capped on it at all.
func ClaimLimit(namespace, poolName string) (int64, bool, error) {
	limit, capped, err := claim.Limit(namespace, "pool", poolName)
	if err != nil || !capped {
		return 0, false, err
	}
	size, err := sizeconv.FromSize(limit)
	if err != nil {
		return 0, false, fmt.Errorf("%s claim on the %s pool: %w", namespace, poolName, err)
	}
	return size, true, nil
}

// ClaimHeldByPath is what each volume of a namespace claims of a pool,
// counting the size it was created or resized with.
//
// It counts what was asked for, not what is written: a pool hands out what it
// promised, and that promise is what is being rationed.
//
// It is answered by object rather than as a sum because a claim granted a
// moment ago and not yet written has to be weighed against it, and weighing
// the two means knowing which object each is about.
func ClaimHeldByPath(ctx context.Context, c *client.T, namespace, poolName string) (map[string]int64, error) {
	name := api.InQueryPoolName(poolName)
	resp, err := c.GetPoolVolumesWithResponse(ctx, &api.GetPoolVolumesParams{Name: &name})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("read the %s pool volumes: unexpected status code %d", poolName, resp.StatusCode())
	}
	held := make(map[string]int64)
	for _, item := range resp.JSON200.Items {
		p, err := naming.ParsePath(item.Path)
		if err != nil {
			continue
		}
		if p.Namespace != namespace {
			continue
		}
		held[item.Path] = item.Size
	}
	return held, nil
}

// ClaimFits says whether a namespace may have an object hold size bytes of a
// pool, and why not when it may not.
//
// The size is what the object is to hold, not the increase: what it holds
// today is already counted in what the namespace holds, and is not claimed
// twice.
//
// A namespace with no claim on the pool is not capped on it, and is answered
// from the local configuration alone, which is what most allocations are.
//
// A capped one is brokered. The claim is asked of the daemon, which hands the
// question to the node speaking for the cluster: what the namespace holds is
// read from the configurations the cluster shares, a write reaches that
// reading a moment after it is made, and two claims answered from the same
// reading both fit where together they do not. One node answering them,
// counting what it has granted since, is what makes them fit one at a time.
//
// Failing to reach the daemon leaves the claim unchecked rather than refused:
// a cap is something the cluster brokers, and where there is no daemon to ask
// there is nothing brokering. Allocating with the daemon down is an
// administrator acting directly, which is uncapped by design, and stopping a
// daemon is not something the capped user can do.
func ClaimFits(ctx context.Context, namespace, poolName, path string, size int64) (bool, string, error) {
	if _, capped, err := ClaimLimit(namespace, poolName); err != nil || !capped {
		// The common case, and it asked nothing of the daemon.
		return true, "", nil
	}
	c, err := client.New()
	if err != nil {
		return true, "", nil
	}
	resp, err := c.PostPoolClaimWithResponse(ctx, api.PostPoolClaim{
		Namespace: namespace,
		Path:      &path,
		Pool:      poolName,
		Size:      size,
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

// UsageByNode is what each node holds of a pool, as the daemon knows it.
//
// The pool of a node is read from the node itself, and a node cannot read the
// pool of a peer, so this is asked of the daemon: it is where every node
// reports what its storage holds. A node reporting nothing is absent from the
// answer rather than present with zeroes, because a pool with no room and a
// pool nobody has heard from are not the same thing.
func UsageByNode(ctx context.Context, poolName string) (map[string]Usage, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	name := api.InQueryPoolName(poolName)
	selector := "*"
	resp, err := c.GetPoolsWithResponse(ctx, &api.GetPoolsParams{Name: &name, Node: &selector})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("read the %s pool usage: unexpected status code %d", poolName, resp.StatusCode())
	}
	m := make(map[string]Usage)
	for _, item := range resp.JSON200.Items {
		if item.Name != poolName {
			continue
		}
		m[item.Node] = Usage{
			Shared:      item.Shared,
			Free:        item.Free,
			Used:        item.Used,
			Size:        item.Size,
			LogicalFree: item.LogicalFree,
			LogicalUsed: item.LogicalUsed,
			LogicalSize: item.LogicalSize,
		}
	}
	return m, nil
}
