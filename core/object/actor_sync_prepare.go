package object

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/plog"
)

// quiesceGroups are the driver groups of the resources a grouped snapshot of
// the sync resources quiesces, as a stop orders them.
var quiesceGroups = []driver.Group{
	driver.GroupIP, driver.GroupVolume, driver.GroupDisk, driver.GroupFS, driver.GroupShare,
	driver.GroupContainer, driver.GroupApp, driver.GroupSync, driver.GroupTask,
}

// prepareSyncReleaseTimeout bounds the release of what the sync resources
// took, as their snapshots, run with a context of its own once the action is
// done, whatever became of its context.
const prepareSyncReleaseTimeout = 10 * time.Minute

// prepareSync has the selected sync resources of a sync update or full
// action take what their transfers copy from, as their snapshots, before the
// first transfer, under one quiesce of the object: their copies are of one
// point in time, as v2 made them. It returns how to release it all, once
// the action is done.
//
// A resource with a configuration error, or run in a container rather than
// on this node, takes nothing here, as it runs nothing here.
func (t *actor) prepareSync(ctx context.Context, selected resource.Drivers) (func(), error) {
	l := make([]resource.Driver, 0)
	for _, r := range selected {
		if r.GetConfigurationError() != nil {
			continue
		}
		if v, err := t.isEncapNodeMatchingResource(r); err != nil || !v {
			continue
		}
		l = append(l, r)
	}
	quiesce := func(ctx context.Context) (func(context.Context) error, error) {
		rs := t.ResourcesByDrivergroups(quiesceGroups)
		rs.Reverse()
		return resource.QuiesceAll(ctx, rs)
	}
	if left := syncsLeftOut(ctx, l, t.ResourcesByDrivergroups([]driver.Group{driver.GroupSync})); len(left) > 0 {
		t.log.Infof("the syncs %s take their snapshots too, not in this run: theirs are not of this moment", strings.Join(left, " "))
	}
	return prepareSyncs(ctx, t.log, l, quiesce)
}

// syncsLeftOut returns the sync resources of all that would take a snapshot
// before the transfers, as the selected ones of an action do, but are not
// selected: a run of a part of them takes its snapshots apart from theirs.
// None when no selected one takes a snapshot.
func syncsLeftOut(ctx context.Context, selected []resource.Driver, all []resource.Driver) []string {
	isSelected := make(map[string]bool, len(selected))
	preparing := false
	for _, r := range selected {
		isSelected[r.RID()] = true
		preparing = preparing || resource.WillPrepareSync(ctx, r)
	}
	if !preparing {
		return nil
	}
	l := make([]string, 0)
	for _, r := range all {
		if !isSelected[r.RID()] && resource.WillPrepareSync(ctx, r) {
			l = append(l, r.RID())
		}
	}
	return l
}

// prepareSyncs has the resources of l that will take something before the
// transfers take it, under quiesce, called once and only when one of them
// will, and returns how to release what they took, the last taken first.
//
// A resource failing to take it fails the action before any transfer,
// releasing what the others took, unless it is optional: it is warned about
// and left out, its own transfer taking it again or failing as it does
// without the grouping.
func prepareSyncs(ctx context.Context, log *plog.Logger, l []resource.Driver, quiesce func(context.Context) (func(context.Context) error, error)) (func(), error) {
	preparers := make([]resource.Driver, 0)
	for _, r := range l {
		if resource.WillPrepareSync(ctx, r) {
			preparers = append(preparers, r)
		}
	}
	if len(preparers) == 0 {
		return func() {}, nil
	}
	releases := make([]func() error, 0, len(preparers))
	rids := make([]string, 0, len(preparers))
	release := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			if err := releases[i](); err != nil {
				log.Warnf("%s: release: %s", rids[i], err)
			}
		}
	}
	resume, err := quiesce(ctx)
	if err != nil {
		return nil, fmt.Errorf("quiesce before the snapshots of the syncs: %w", err)
	}
	var errs error
	for _, r := range preparers {
		f, err := resource.PrepareSync(ctx, r)
		if err != nil {
			if r.IsOptional() {
				r.Log().Warnf("prepare the sync, the resource is optional: %s", err)
				continue
			}
			errs = fmt.Errorf("%s: prepare the sync: %w", r.RID(), err)
			break
		}
		if f != nil {
			releases = append(releases, f)
			rids = append(rids, r.RID())
		}
	}
	if err := resume(ctx); err != nil {
		errs = errors.Join(errs, fmt.Errorf("resume after the snapshots of the syncs: %w", err))
	}
	if errs != nil {
		release()
		return nil, errs
	}
	if len(rids) > 1 {
		log.Infof("the syncs %v copy from snapshots taken at one moment", rids)
	}
	return release, nil
}
