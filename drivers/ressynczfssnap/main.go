package ressynczfs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/ressync"
	"github.com/opensvc/om3/v3/util/zfs"
)

// T is the driver structure.
type (
	T struct {
		ressync.T
		Dataset   []string
		Recursive bool
		Keep      int
		Name      string
	}

	modeT uint
)

const (
	modeFull modeT = iota
	modeIncr

	timeFormatInSnapName = "2006-01-02.15:04:05"
)

func New() resource.Driver {
	return &T{}
}

func (t *T) SortKey() string {
	// The "+" ascii char is ordered before any rfc952 char, so using it
	// as a prefix in the sort key makes sure it is ordered before any
	// driver using t.ResourceID.Name as its sort key (which is the
	// default).
	return "+" + t.ResourceID.Name
}

func (t *T) Update(ctx context.Context) error {
	if v, reason := t.IsInstanceSufficientlyStarted(ctx); !v {
		t.Log().Tracef("the instance is not sufficiently started (%s). refuse to create snapshots", reason)
		return nil
	}
	done, err := t.StartRun()
	if err != nil {
		return err
	}
	defer done()
	for _, dataset := range t.Dataset {
		if err := t.createSnap(dataset); err != nil {
			return err
		}
		if err := t.removeSnap(dataset); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) removeSnap(dataset string) error {
	datasets, err := zfs.ListFilesystems(
		zfs.ListWithNames(dataset),
		zfs.ListWithOrderBy("creation"),
		zfs.ListWithOrderReverse(),
		zfs.ListWithTypes(zfs.DatasetTypeSnapshot),
		zfs.ListWithLogger(t.Log()),
	)
	if err != nil {
		return err
	}
	kept := 0
	expectedPrefix := t.snapPrefix(dataset)
	for _, candidate := range datasets {
		if !strings.HasPrefix(candidate.Name, expectedPrefix) {
			continue
		}
		if kept < t.Keep {
			kept++
			t.Log().Tracef("keep snap %s %d/%d", candidate.Name, kept, t.Keep)
			continue
		}
		// The snapshot was taken in the descendant datasets too when
		// recursive, and goes from them too.
		if err := candidate.Destroy(zfs.FilesystemDestroyWithRemoveSnapshots(t.Recursive)); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) status(ctx context.Context, dataset string, isSource bool, notSourceReason string) status.T {
	var rep ressync.DatasetReplicator
	if !isSource {
		if rep = t.replicator(dataset); rep == nil {
			t.StatusLog().Info("%s: no snapshot is taken here (%s), and none is replicated here", dataset, notSourceReason)
			return status.NotApplicable
		}
	}
	datasets, err := zfs.ListFilesystems(
		zfs.ListWithNames(dataset),
		zfs.ListWithOrderBy("creation"),
		zfs.ListWithOrderReverse(),
		zfs.ListWithTypes(zfs.DatasetTypeSnapshot),
		zfs.ListWithLogger(t.Log()),
	)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return status.NotApplicable
		}
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	kept := 0
	snapCount := 0
	issueCount := 0
	expectedPrefix := t.snapPrefix(dataset)
	for _, candidate := range datasets {
		if !strings.HasPrefix(candidate.Name, expectedPrefix) {
			continue
		}
		snapCount++
		if kept < t.Keep {
			kept++
		}
		if kept == 1 {
			timeStr := candidate.Name[len(expectedPrefix):]
			createdAt, err := time.ParseInLocation(timeFormatInSnapName, timeStr, time.Local)
			if err != nil {
				t.StatusLog().Error("%s", err)
				issueCount++
				continue
			}
			maxDelay := t.GetMaxDelay(createdAt)
			if maxDelay == 0 {
				continue
			}
			origin := t.MaxDelayOrigin()
			if rep != nil {
				// A replication with no delay to keep says nothing
				// of how old the replicas may be.
				repDelay := rep.GetMaxDelay(createdAt)
				if repDelay == 0 {
					continue
				}
				maxDelay += repDelay
				origin = fmt.Sprintf("%s; plus the replication by %s, %s", origin, rep.RID(), rep.MaxDelayOrigin())
			}
			// The snapshot goes stale then, with no event to tell.
			t.StatusLog().ChangesAt(createdAt.Add(maxDelay))
			age := time.Since(createdAt)
			if age > maxDelay {
				t.StatusLog().Warn("%s last snap is too old, created at %s, more than %s ago (%s)", t.Name, createdAt, maxDelay, origin)
				issueCount++
			}
		}
	}
	if snapCount == 0 {
		t.StatusLog().Warn("%s has no snap", t.Name)
		issueCount++
	} else if n := snapCount - t.Keep; n > 0 {
		t.StatusLog().Warn("%s has %d too many snaps", t.Name, n)
		issueCount++
	}
	if issueCount > 0 {
		return status.Warn
	}
	return status.Up
}

// Status reports the snapshots of each dataset.
//
// On the node taking them, the newest is judged by the delay of this
// resource. On another node, the snapshots are the ones a sync resource
// replicated, and the newest is judged by the delay of this resource plus the
// one of that replication: both on time, it is not older. A node taking no
// snapshot and receiving none has none to judge.
func (t *T) Status(ctx context.Context) status.T {
	isSource, reason := t.IsInstanceSufficientlyStarted(ctx)
	var aggSt status.T
	for _, dataset := range t.Dataset {
		st := t.status(ctx, dataset, isSource, reason)
		aggSt.Add(st)
	}
	return aggSt
}

// replicator is the sync resource of the object replicating dataset to the
// peers, nil when none does.
func (t *T) replicator(dataset string) ressync.DatasetReplicator {
	for _, r := range t.GetObjectDriver().ResourcesByDrivergroups([]driver.Group{driver.GroupSync}) {
		if rep, ok := r.(ressync.DatasetReplicator); ok && rep.ReplicatesDataset(dataset) {
			return rep
		}
	}
	return nil
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	if t.Name != "" {
		return fmt.Sprintf("%s of %s", t.Name, strings.Join(t.Dataset, " "))
	} else {
		return fmt.Sprintf("of %s", strings.Join(t.Dataset, " "))
	}
}

func (t *T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action:                   "update",
		Option:                   "schedule",
		Base:                     "",
		RequireReplicationSource: true,
		Require:                  t.UpdateRequires,
	}
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) zfs(name string) *zfs.Filesystem {
	return &zfs.Filesystem{Name: name, Log: t.Log()}
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "dataset", Value: strings.Join(t.Dataset, " ")},
		{Key: "name", Value: t.Name},
		{Key: "keep", Value: fmt.Sprintf("%d", t.Keep)},
		{Key: "recursive", Value: fmt.Sprintf("%v", t.Recursive)},
		{Key: "max_delay", Value: fmt.Sprintf("%s", t.MaxDelay)},
		{Key: "schedule", Value: t.Schedule},
	}
	return m, nil
}

func (t *T) createSnap(dataset string) error {
	snapName := t.snapName(dataset)
	if err := t.zfs(snapName).Snapshot(zfs.FilesystemSnapshotWithRecursive(t.Recursive)); err != nil {
		return err
	}
	return nil
}

func (t *T) snapPrefix(dataset string) string {
	return fmt.Sprintf("%s@%s.snap.", dataset, t.Name)
}

func (t *T) snapName(dataset string) string {
	dateStr := time.Now().Format(timeFormatInSnapName)
	return fmt.Sprintf("%s%s", t.snapPrefix(dataset), dateStr)
}
