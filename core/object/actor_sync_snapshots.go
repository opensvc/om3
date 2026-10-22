package object

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// RestoredConfig returns the configuration the selected sync resource holds
// at the snapshot the restore options of the context name, as the object
// restores it, and the keywords it changes. One resource holding
// configurations is selected, a --rid naming it among several.
func (t *actor) RestoredConfig(ctx context.Context) ([]byte, []string, error) {
	selector := resourceselector.New(t, resourceselector.WithRID(actioncontext.RID(ctx)))
	restorers := make([]resource.ConfigRestorer, 0)
	rids := make([]string, 0)
	for _, r := range selector.Resources() {
		restorer, ok := r.(resource.ConfigRestorer)
		if !ok || r.IsDisabled() {
			continue
		}
		restorers = append(restorers, restorer)
		rids = append(rids, r.RID())
	}
	switch len(restorers) {
	case 0:
		return nil, nil, fmt.Errorf("no sync resource holds the configuration of the object")
	case 1:
		return restorers[0].RestoredConfig(ctx)
	default:
		return nil, nil, fmt.Errorf("the configuration is held by %s: name the one to restore it from with --rid", strings.Join(rids, " "))
	}
}
