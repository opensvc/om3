package ressynczfs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/ressync"
	"github.com/opensvc/om3/util/zfs"
)

// T is the driver structure.
type (
	T struct {
		ressync.T
		Dataset   []string
		Schedule  string
		Recursive bool
		Keep      int
		Name      string
	}

	modeT uint
)

const (
	modeFull modeT = iota
	modeIncr

	lockName             = "sync"
	timeFormatInSnapName = "2006-01-02.15:04:05"
)

func New() resource.Driver {
	return &T{}
}

func (t T) SortKey() string {
	// The "+" ascii char is ordered before any rfc952 char, so using it
	// as a prefix in the sort key makes sure it is ordered before any
	// driver using t.ResourceID.Name as its sort key (which is the
	// default).
	return "+" + t.ResourceID.Name
}

func (t T) IsRunning() bool {
	unlock, err := t.Lock(false, time.Second*0, lockName)
	if err != nil {
		return true
	}
	defer unlock()
	return false
}

func (t T) Update(ctx context.Context) error {
	if v, rids := t.IsInstanceSufficientlyStarted(ctx); !v {
		t.Log().Debugf("the instance is not sufficiently started (%s). refuse to create snapshots", strings.Join(rids, ","))
		return nil
	}
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

func (t T) removeSnap(dataset string) error {
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
			t.Log().Debugf("keep snap %s %d/%d", candidate.Name, kept, t.Keep)
			continue
		}
		if err := candidate.Destroy(); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) status(ctx context.Context, dataset string) status.T {
	datasets, err := zfs.ListFilesystems(
		zfs.ListWithNames(dataset),
		zfs.ListWithOrderBy("creation"),
		zfs.ListWithOrderReverse(),
		zfs.ListWithTypes(zfs.DatasetTypeSnapshot),
		zfs.ListWithLogger(t.Log()),
	)
	if err != nil {
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
			if maxDelay == nil {
				continue
			}
			age := time.Since(createdAt)
			if age > *maxDelay {
				t.StatusLog().Warn("%s last snap is too old, created at %s (>%s ago)", t.Name, createdAt, *t.MaxDelay)
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

func (t *T) Status(ctx context.Context) status.T {
	var aggSt status.T
	for _, dataset := range t.Dataset {
		st := t.status(ctx, dataset)
		aggSt.Add(st)
	}
	return aggSt
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t T) Label(_ context.Context) string {
	if t.Name != "" {
		return fmt.Sprintf("%s of %s", t.Name, strings.Join(t.Dataset, " "))
	} else {
		return fmt.Sprintf("of %s", strings.Join(t.Dataset, " "))
	}
}

func (t T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action: "sync_update",
		Option: "schedule",
		Base:   "",
	}
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) zfs(name string) *zfs.Filesystem {
	return &zfs.Filesystem{Name: name, Log: t.Log()}
}

func (t T) Info(ctx context.Context) (resource.InfoKeys, error) {
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
