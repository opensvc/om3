package daemonapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/claim"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
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
func refuseClaimOverrun(ctx context.Context, p naming.Path, o any, cfg *xconfig.T) error {
	if err := refuseComputeClaimOverrun(ctx, p, o); err != nil {
		return err
	}
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

// refuseComputeClaimOverrun stops a configuration write taking the namespace
// over what it claimed of the cpu and memory.
//
// An object claims what its processes are capped to, on every instance it
// may run at once, so what a write changes of the caps, of the resources or
// of the flex target is what it claims. The configuration write is where the
// claim is taken, as for a pool: an action runs what the configuration
// already promised.
//
// Only the types the write grows the claim of have to fit: a write leaving an
// object claiming what it did, or less, takes nothing more of the namespace,
// whatever the namespace holds, so an object created before its namespace
// claimed anything can still be written.
func refuseComputeClaimOverrun(ctx context.Context, p naming.Path, o any) error {
	claimer, ok := o.(object.ComputeClaimer)
	if !ok || p.Namespace == naming.NsRoot {
		return nil
	}
	to, err := claimer.ComputeClaims()
	if err != nil {
		return err
	}
	from := configuredComputeClaims(p)
	grown := make(map[string]int64)
	for claimType, v := range to {
		was, ok := from[claimType]
		switch {
		case !ok:
		case was == claim.Unbounded:
			continue
		case v != claim.Unbounded && v <= was:
			continue
		}
		grown[claimType] = v
	}
	if len(grown) == 0 {
		return nil
	}
	ok, why, err := claim.ComputeFits(ctx, p.Namespace, p.String(), grown)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s claims %s, and %s", ErrClaimOverrun, p, object.DescribeComputeClaims(to), why)
	}
	return nil
}
