package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	CmdObjectResize struct {
		OptsGlobal
		commoncmd.OptsAsync
		Size string
	}
)

func (t *CmdObjectResize) Run(kind string) error {
	if t.Size == "" {
		return fmt.Errorf("a size is required, as an argument or with --size")
	}
	change, err := sizeconv.ParseChange(t.Size)
	if err != nil {
		return err
	}
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	c, err := client.New()
	if err != nil {
		return err
	}
	paths, err := objectselector.New(mergedSelector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		return err
	}
	if len(paths) != 1 {
		return fmt.Errorf("%s matches %d objects: a resize is asked of one", mergedSelector, len(paths))
	}
	p := paths[0]

	to, err := t.target(p, change)
	if err != nil {
		return err
	}

	// Growing is claiming more of the pool, so it is checked the way an
	// allocation is. What the volume already holds is counted in, so only
	// what it asks for on top has to fit.
	if err := t.claimFits(p, to); err != nil {
		return err
	}

	// The size the object is configured to hold is what every node converges
	// to, so it is written before the orchestration is asked for. Writing it
	// is also what keeps a listing like "om pool volume ls" honest.
	if err := t.setConfiguredSize(c, p, to); err != nil {
		return err
	}

	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithAsyncTarget("resized"),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithSort(t.Sort),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
	).Do()
}

// target is the size to reach, and refuses a shrink.
//
// A shrink is left out of the orchestration deliberately: it is not safe to
// retry, and its order through the chain is the reverse of a grow, so it is
// asked of one instance at a time instead.
func (t *CmdObjectResize) target(p naming.Path, change sizeconv.Change) (int64, error) {
	from, err := t.configuredSize(p)
	switch {
	case err != nil && change.IsRelative:
		return 0, fmt.Errorf("%s: an amount to add or remove is resolved against the configured size: %w", p, err)
	case err != nil:
		// Nothing to resolve against and nothing to compare to. The daemons
		// still refuse what they cannot do.
		return change.Value, nil
	}
	to := change.Resolve(from)
	if to <= from {
		return 0, fmt.Errorf("%s is configured to hold %s: an orchestrated resize only grows. Use \"om %s instance resize\" on each node to shrink",
			p, sizeconv.BSizeCompact(float64(from)), p)
	}
	return to, nil
}

// claimFits refuses a grow the namespace has no room for in the pool serving
// the volume. A volume served by no pool is claimed from nothing and capped by
// nothing.
func (t *CmdObjectResize) claimFits(p naming.Path, to int64) error {
	type poolNamer interface {
		PoolName() (string, error)
	}
	o, err := object.New(p, object.WithVolatile(true))
	if err != nil {
		return err
	}
	i, ok := o.(poolNamer)
	if !ok {
		return nil
	}
	poolName, err := i.PoolName()
	if err != nil || poolName == "" {
		return nil
	}
	from, err := t.configuredSize(p)
	if err != nil {
		return err
	}
	if to <= from {
		return nil
	}
	ok, why, err := pool.ClaimFits(context.Background(), p.Namespace, poolName, to-from)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s is served by the %s pool, and %s", p, poolName, why)
	}
	return nil
}

func (t *CmdObjectResize) configuredSize(p naming.Path) (int64, error) {
	type configuredSizer interface {
		ConfiguredSize() (int64, error)
	}
	o, err := object.New(p, object.WithVolatile(true))
	if err != nil {
		return 0, err
	}
	i, ok := o.(configuredSizer)
	if !ok {
		return 0, fmt.Errorf("a %s has no configured size", p.Kind)
	}
	return i.ConfiguredSize()
}

func (t *CmdObjectResize) setConfiguredSize(c *client.T, p naming.Path, to int64) error {
	set := []string{fmt.Sprintf("size=%d", to)}
	params := api.PatchObjectConfigParams{Set: &set}
	resp, err := c.PatchObjectConfigWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("%s: set size: unexpected status code %d", p, resp.StatusCode())
	}
	return nil
}
