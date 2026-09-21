package daemonapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

// ErrClaimOverrun is a configuration write taking the namespace over what it
// claimed of a cluster resource.
var ErrClaimOverrun = errors.New("claim overrun")

// refuseClaimOverrun stops a configuration write taking the namespace over
// what it claimed of the pool serving the object.
//
// The claim on a pool counts the size each volume of the namespace is
// configured to hold, so the configuration write is where the claim is taken:
// an action growing the storage afterwards only carries out the size the
// configuration already promised. Guarding the action alone leaves a client
// free to write the promise and then ask for it to be honoured.
//
// Only the increase has to fit. What the object is already configured to hold
// is counted in what the namespace holds, so it is not claimed twice: the
// size the object is to hold is what is asked about, and the claim weighs it
// against what the namespace holds of the pool elsewhere.
func refuseClaimOverrun(ctx context.Context, p naming.Path, cfg *xconfig.T) error {
	poolName := cfg.GetString(key.T{Section: "DEFAULT", Option: "pool"})
	if poolName == "" {
		// Served by no pool, so claimed from nothing and capped by nothing.
		return nil
	}
	to := cfg.GetSize(key.T{Section: "DEFAULT", Option: "size"})
	if to == nil {
		return nil
	}
	// An object this node has no configuration for is one being created, and
	// it holds nothing yet.
	from, err := configuredSize(p)
	if err != nil {
		from = 0
	}
	if *to <= from {
		return nil
	}
	ok, why, err := pool.ClaimFits(ctx, p.Namespace, poolName, p.String(), *to)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s is served by the %s pool, and %s", ErrClaimOverrun, p, poolName, why)
	}
	return nil
}
