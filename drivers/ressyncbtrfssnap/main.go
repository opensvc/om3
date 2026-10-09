// Package ressyncbtrfssnap is the sync.btrfssnap driver: it keeps a number
// of read-only snapshots of btrfs subvolumes, in their .snap directory, where
// the users of the subvolume see them.
package ressyncbtrfssnap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/ressync"
	"github.com/opensvc/om3/v3/util/btrfs"
)

type (
	// T is the driver structure.
	T struct {
		ressync.T
		Subvol    []string
		Keep      int
		Name      string
		Recursive bool
	}

	// snapshot is a snapshot of the resource in a .snap directory.
	snapshot struct {
		Path      string
		CreatedAt time.Time
	}
)

const (
	snapDir = ".snap"

	// timeFormat is the format of the time a snapshot name starts with,
	// the UTC time it was taken at, as v2 named them.
	timeFormat = "2006-01-02T15:04:05.000000Z"
)

var (
	// timeFormats are the formats of the times in the snapshot names read:
	// v2 left the fraction of second out when it was zero.
	timeFormats = []string{timeFormat, "2006-01-02T15:04:05Z"}

	// readMountinfo returns the mounts of the node, replaced by the tests.
	readMountinfo = func() ([]byte, error) {
		return os.ReadFile("/proc/self/mountinfo")
	}
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

// snapName is the name of a snapshot taken at now.
func (t *T) snapName(now time.Time) string {
	s := now.UTC().Format(timeFormat)
	if t.Name != "" {
		s += "," + t.Name
	}
	return s
}

// parseSnapName returns the time a snapshot of the resource was taken at,
// from its name, and false for the name of another snapshot.
func (t *T) parseSnapName(name string) (time.Time, bool) {
	s := name
	if t.Name != "" {
		var ok bool
		if s, ok = strings.CutSuffix(name, ","+t.Name); !ok {
			return time.Time{}, false
		}
	} else if strings.Contains(name, ",") {
		return time.Time{}, false
	}
	for _, format := range timeFormats {
		if tm, err := time.Parse(format, s); err == nil {
			return tm, true
		}
	}
	return time.Time{}, false
}

// snapshots returns the snapshots of the resource in the .snap directory of
// the subvolume mounted at dir, the newest first.
func (t *T) snapshots(dir string) ([]snapshot, error) {
	entries, err := os.ReadDir(filepath.Join(dir, snapDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	l := make([]snapshot, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		tm, ok := t.parseSnapName(e.Name())
		if !ok {
			continue
		}
		l = append(l, snapshot{Path: filepath.Join(dir, snapDir, e.Name()), CreatedAt: tm})
	}
	sort.Slice(l, func(i, j int) bool { return l[i].CreatedAt.After(l[j].CreatedAt) })
	return l, nil
}

// dirs returns the directories the writable subvolumes of loc are mounted at
// on this node, the subvolume first, the nested ones after when recursive,
// and false when the subvolume is not mounted.
func (t *T) dirs(loc btrfs.Location) ([]string, bool, error) {
	mi, err := readMountinfo()
	if err != nil {
		return nil, false, err
	}
	mnt, ok := btrfs.MountOfSubvol(mi, loc.Subvol, false)
	if !ok {
		return nil, false, nil
	}
	l := []string{mnt}
	if !t.Recursive {
		return l, true, nil
	}
	all, err := listSubvols(mnt, false)
	if err != nil {
		return nil, true, err
	}
	ro, err := listSubvols(mnt, true)
	if err != nil {
		return nil, true, err
	}
	readOnly := make(map[int64]bool, len(ro))
	for _, s := range ro {
		readOnly[s.ID] = true
	}
	nested := make([]string, 0)
	for _, s := range all {
		rel, ok := strings.CutPrefix(s.Path, loc.Subvol+"/")
		if !ok || readOnly[s.ID] {
			continue
		}
		nested = append(nested, rel)
	}
	sort.Strings(nested)
	for _, rel := range nested {
		l = append(l, filepath.Join(mnt, rel))
	}
	return l, true, nil
}

func listSubvols(mnt string, readOnly bool) ([]btrfs.Subvol, error) {
	b, err := btrfsCmd(btrfs.ListArgs(readOnly, mnt)...)
	if err != nil {
		return nil, err
	}
	return btrfs.ParseList(b)
}

// btrfsCmd runs the btrfs command, replaced by the tests.
var btrfsCmd = func(args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("btrfs", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", cmd, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
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
	var errs error
	now := time.Now()
	for _, s := range t.Subvol {
		loc, err := btrfs.ParseLocation(s)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		dirs, mounted, err := t.dirs(loc)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", loc, err))
			continue
		}
		if !mounted {
			t.Log().Infof("%s: not mounted, no snapshot taken", loc)
			continue
		}
		for _, dir := range dirs {
			if err := t.snapAndPrune(dir, now); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}
	return errs
}

// snapAndPrune takes a snapshot of the subvolume mounted at dir, and deletes
// its oldest snapshots beyond keep.
func (t *T) snapAndPrune(dir string, now time.Time) error {
	if err := os.MkdirAll(filepath.Join(dir, snapDir), 0755); err != nil {
		return err
	}
	snap := filepath.Join(dir, snapDir, t.snapName(now))
	t.Log().Infof("btrfs subvolume snapshot -r %s %s", dir, snap)
	if _, err := btrfsCmd("subvolume", "snapshot", "-r", dir, snap); err != nil {
		return err
	}
	l, err := t.snapshots(dir)
	if err != nil {
		return err
	}
	keep := max(t.Keep, 1)
	if len(l) <= keep {
		return nil
	}
	var errs error
	for _, s := range l[keep:] {
		t.Log().Infof("btrfs subvolume delete %s", s.Path)
		if _, err := btrfsCmd("subvolume", "delete", s.Path); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

// Status reports the snapshots of each subvolume, on the node the instance
// is started on: the snapshots are not replicated, so another node takes and
// holds none.
func (t *T) Status(ctx context.Context) status.T {
	if v, reason := t.IsInstanceSufficientlyStarted(ctx); !v {
		t.StatusLog().Info("no snapshot is taken here (%s)", reason)
		return status.NotApplicable
	}
	var agg status.T
	for _, s := range t.Subvol {
		agg.Add(t.status(s))
	}
	return agg
}

func (t *T) status(s string) status.T {
	loc, err := btrfs.ParseLocation(s)
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	dirs, mounted, err := t.dirs(loc)
	if err != nil {
		t.StatusLog().Error("%s: %s", loc, err)
		return status.Undef
	}
	if !mounted {
		t.StatusLog().Info("%s is not mounted", loc)
		return status.NotApplicable
	}
	var agg status.T
	for _, dir := range dirs {
		agg.Add(t.statusDir(dir))
	}
	return agg
}

func (t *T) statusDir(dir string) status.T {
	l, err := t.snapshots(dir)
	if err != nil {
		t.StatusLog().Error("%s: %s", dir, err)
		return status.Undef
	}
	if len(l) == 0 {
		t.StatusLog().Warn("%s has no snap", dir)
		return status.Warn
	}
	result := status.Up
	if n := len(l) - max(t.Keep, 1); n > 0 {
		t.StatusLog().Warn("%s has %d too many snaps", dir, n)
		result = status.Warn
	}
	newest := l[0].CreatedAt
	if maxDelay := t.GetMaxDelay(newest); maxDelay > 0 {
		// The snapshot goes stale then, with no event to tell.
		t.StatusLog().ChangesAt(newest.Add(maxDelay))
		if time.Since(newest) > maxDelay {
			t.StatusLog().Warn("%s last snap is too old, created at %s, more than %s ago (%s)", dir, newest, maxDelay, t.MaxDelayOrigin())
			result = status.Warn
		}
	}
	return result
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	if t.Name != "" {
		return fmt.Sprintf("%s of %s", t.Name, strings.Join(t.Subvol, " "))
	}
	return fmt.Sprintf("of %s", strings.Join(t.Subvol, " "))
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

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "subvol", Value: strings.Join(t.Subvol, " ")},
		{Key: "name", Value: t.Name},
		{Key: "keep", Value: fmt.Sprintf("%d", t.Keep)},
		{Key: "recursive", Value: fmt.Sprintf("%v", t.Recursive)},
		{Key: "schedule", Value: t.Schedule},
	}
	if t.MaxDelay != nil {
		m = append(m, resource.InfoKey{Key: "max_delay", Value: t.MaxDelay.String()})
	}
	return m, nil
}
