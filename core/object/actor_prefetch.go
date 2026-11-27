package object

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceselector"
)

// Prefetch gets the images the resources of the instance run their
// containers from, when the store of their engine does not hold them: all of
// them, or the ones a resource selector expression selects.
//
// An image is copied from a node of the object holding it, when one does,
// and pulled from its registry when none does: see imageSources.
//
// It is run by the daemon on the nodes not running the instance, ahead of a
// failover, and takes neither the action lock nor the frozen state into
// account: it changes nothing of the instance, and a pull may last for as
// long as an action waiting for the lock would not. A start pulling the same
// image meanwhile is the engine's to arbitrate.
//
// A disabled resource is never started, and an encap one runs in its guest:
// neither is prefetched. A pull that failed is in the report of its
// resource, and recorded on the node: the error returned is of the resources
// whose prefetch could not be tried, as with an engine that does not answer.
func (t *actor) Prefetch(ctx context.Context, rid string) ([]resource.PrefetchReport, error) {
	var errs []error
	reports := make([]resource.PrefetchReport, 0)
	rs := resourceselector.New(t, resourceselector.WithRID(rid))
	var sources func(rid string) []resource.ImageSource
	for _, r := range rs.Resources() {
		prefetcher, ok := r.(resource.Prefetcher)
		if !ok || r.IsDisabled() || r.IsEncap() {
			continue
		}
		if sources == nil {
			// Asked once for the resources of the object, and not
			// for an object with nothing to prefetch.
			sources = t.imageSources(ctx)
		}
		report, err := prefetcher.Prefetch(ctx, sources(r.RID()))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.RID(), err))
			continue
		}
		reports = append(reports, report)
	}
	return reports, errors.Join(errs...)
}
