package omcmd

import (
	"context"
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/instance"
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
	// No size asked for is the size the object is configured to hold, which
	// is what every node converges to. Asking for it again is how a resize
	// that stopped part way is finished.
	var change sizeconv.Change
	var err error
	if t.Size != "" {
		if change, err = sizeconv.ParseChange(t.Size); err != nil {
			return err
		}
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

	// The size is written before the orchestration is asked for, so a resize
	// already running would take this one as its target and reach a size
	// nobody asked it for, while this one is refused for being second.
	if err := t.refuseWhileResizing(c, p); err != nil {
		return err
	}

	// The size the object is configured to hold is what every node converges
	// to, so it is written before the orchestration is asked for. Writing it
	// is also what keeps a listing like "om pool volume ls" honest.
	configUpdatedAt, err := t.setConfiguredSize(c, p, to)
	if err != nil {
		return err
	}

	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithAsyncTargetOptions(instance.MonitorGlobalExpectOptionsResized{
			ConfigUpdatedAt: configUpdatedAt,
		}),
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
	case err != nil && change.Value == 0:
		return 0, fmt.Errorf("%s: a size is required, as an argument or with --size: %w", p, err)
	case err != nil:
		// Nothing to resolve against and nothing to compare to. The daemons
		// still refuse what they cannot do.
		return change.Value, nil
	}
	if change.Value == 0 && !change.IsRelative {
		// Converge to the size already configured, which is how a resize that
		// stopped part way is finished.
		return from, nil
	}
	to := change.Resolve(from)
	if to < from {
		return 0, fmt.Errorf("%s is configured to hold %s: an orchestrated resize only grows. Use \"om %s instance resize\" on each node to shrink",
			p, sizeconv.BSizeCompact(float64(from)), p)
	}
	// to == from is not refused: the configuration is the target, and asking
	// for it again finishes a resize that stopped part way. Every link that
	// holds it already is skipped, so an object that reached it has nothing
	// to do.
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

// setConfiguredSize writes the size every node converges to, and returns the
// timestamp the configuration now carries.
//
// That timestamp is what the orchestration is asked for: the write reaches the
// peer nodes a moment after it is acknowledged here, and a node reading the
// configuration before it lands would grow to the size this one replaces.
func (t *CmdObjectResize) setConfiguredSize(c *client.T, p naming.Path, to int64) (time.Time, error) {
	var updatedAt time.Time
	set := []string{fmt.Sprintf("size=%d", to)}
	params := api.PatchObjectConfigParams{Set: &set}
	resp, err := c.PatchObjectConfigWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
	if err != nil {
		return updatedAt, err
	}
	if resp.StatusCode() != 200 {
		return updatedAt, fmt.Errorf("%s: set size: unexpected status code %d", p, resp.StatusCode())
	}
	if s := resp.HTTPResponse.Header.Get(api.HeaderLastModified); s != "" {
		if v, err := time.Parse(time.RFC3339Nano, s); err == nil {
			updatedAt = v
		}
	}
	return updatedAt, nil
}

// refuseWhileResizing stops a resize asked of an object already resizing.
//
// The configured size is the target every node reads, and it is written
// before the orchestration is asked for. A resize running at that moment
// reads the new target as its own, so it grows to a size its own request
// never named, and this one is then refused for finding an orchestration in
// progress. The write is what has to be refused, not the request after it.
func (t *CmdObjectResize) refuseWhileResizing(c *client.T, p naming.Path) error {
	params := api.GetInstancesParams{}
	selector := p.String()
	params.Path = &selector
	resp, err := c.GetInstancesWithResponse(context.Background(), &params)
	if err != nil || resp.JSON200 == nil {
		// The daemon is what runs an orchestration. Where none answers, none
		// is running.
		return nil
	}
	for _, item := range resp.JSON200.Items {
		if item.Data.Monitor.GlobalExpect == instance.MonitorGlobalExpectResized {
			return fmt.Errorf("%s is already resizing, asked of %s: wait for it to end, or abort it with \"om %s abort\"",
				p, item.Meta.Node, p)
		}
	}
	return nil
}
