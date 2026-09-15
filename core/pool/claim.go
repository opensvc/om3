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

// ClaimHeld is what a namespace already claims of a pool, counting the size
// each of its volumes was created or resized with.
//
// It counts what was asked for, not what is written: a pool hands out what it
// promised, and that promise is what is being rationed.
func ClaimHeld(ctx context.Context, c *client.T, namespace, poolName string) (int64, error) {
	name := api.InQueryPoolName(poolName)
	resp, err := c.GetPoolVolumesWithResponse(ctx, &api.GetPoolVolumesParams{Name: &name})
	if err != nil {
		return 0, err
	}
	if resp.JSON200 == nil {
		return 0, fmt.Errorf("read the %s pool volumes: unexpected status code %d", poolName, resp.StatusCode())
	}
	var held int64
	for _, item := range resp.JSON200.Items {
		p, err := naming.ParsePath(item.Path)
		if err != nil {
			continue
		}
		if p.Namespace != namespace {
			continue
		}
		held += item.Size
	}
	return held, nil
}

// ClaimFits says whether a namespace may take size more of a pool, and why not
// when it may not.
//
// A namespace with no claim on the pool is not capped on it.
// A namespace claiming nothing is answered from the local configuration
// alone, which is what most allocations are.
//
// What the namespace already holds has to be counted from the volumes the
// whole cluster knows, so that one is read through the daemon. Failing to
// reach it leaves the claim unchecked rather than refused: a cap is something
// the cluster brokers, and where there is no daemon to ask there is nothing
// brokering. Allocating with the daemon down is an administrator acting
// directly, which is uncapped by design, and stopping a daemon is not
// something the capped user can do.
func ClaimFits(ctx context.Context, namespace, poolName string, size int64) (bool, string, error) {
	limit, capped, err := ClaimLimit(namespace, poolName)
	if err != nil {
		return true, "", nil
	}
	if !capped {
		// The common case, and it asked nothing of the daemon.
		return true, "", nil
	}
	c, err := client.New()
	if err != nil {
		return true, "", nil
	}
	held, err := ClaimHeld(ctx, c, namespace, poolName)
	if err != nil {
		return true, "", nil
	}
	if held+size <= limit {
		return true, "", nil
	}
	return false, fmt.Sprintf("the %s namespace may claim %s of it and already claims %s, so it cannot claim %s more",
		namespace,
		sizeconv.BSizeCompact(float64(limit)),
		sizeconv.BSizeCompact(float64(held)),
		sizeconv.BSizeCompact(float64(size))), nil
}
