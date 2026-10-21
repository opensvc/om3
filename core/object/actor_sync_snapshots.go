package object

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceselector"
)

// SyncSnapshot is a point in time a sync resource of the object can restore.
type SyncSnapshot struct {
	RID string `json:"rid"`
	resource.Snapshot
}

// SyncSnapshots returns the points in time the selected sync resources can
// restore, the resources holding none skipped.
func (t *actor) SyncSnapshots(ctx context.Context) ([]SyncSnapshot, error) {
	l := make([]SyncSnapshot, 0)
	var errs error
	// A listing, not an action: the selector is the rid of the context
	// alone, there being no action properties to order it by.
	selector := resourceselector.New(t, resourceselector.WithRID(actioncontext.RID(ctx)))
	for _, r := range selector.Resources() {
		lister, ok := r.(resource.SnapshotLister)
		if !ok || r.IsDisabled() {
			continue
		}
		snapshots, err := lister.Snapshots(ctx)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", r.RID(), err))
			continue
		}
		for _, snapshot := range snapshots {
			l = append(l, SyncSnapshot{RID: r.RID(), Snapshot: snapshot})
		}
	}
	return l, errs
}
