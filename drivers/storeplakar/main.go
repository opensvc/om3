// Package storeplakar is the plakar store driver: a Kloset store, a
// directory of the node or a remote location, driven through the plakar
// binary of the node.
//
// Each member of a run is a snapshot, and a completed run has one more, its
// marker. A snapshot is never changed once written, so what a run says is in
// the tags of its snapshots, as om3.<key>=<value>, the value query-escaped:
// a tag holds no space nor comma. The path an object had at its last
// completed run, which a listing by path reads, and the name its cluster had
// then, are the ones of the latest marker.
//
// Every call runs
//
//	plakar -configdir <cfg> -cachedir <cache> -datadir <data> at @store ...
//
// with the passphrase_cmd of @store running om <sec> key decode <key>: the
// passphrase goes through a pipe only, never in a file, the command line or
// the environment.
package storeplakar

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// T is the plakar store driver.
	T struct {
		backup.T
		log *plog.Logger
	}
)

const (
	tagRun         = "om3.run"
	tagMember      = "om3.member"
	tagKind        = "om3.kind"
	tagHead        = "om3.head"
	tagObject      = "om3.object"
	tagPath        = "om3.path"
	tagCluster     = "om3.cluster"
	tagClusterName = "om3.cluster_name"
	tagNode        = "om3.node"
	tagCreated     = "om3.created"
	tagProtected   = "om3.protected"
	tagNote        = "om3.note"
	tagComplete    = "om3.complete"
	tagCompleted   = "om3.completed"

	// markerName is the file the marker of a completed run holds, empty.
	markerName = "complete"
)

// SetLoggerForTest sets the logger the plakar command lines and outputs go
// to.
func (t *T) SetLoggerForTest(log *plog.Logger) {
	t.log = log
}

// Log is the logger of the store, prefixed with its reference.
func (t *T) Log() *plog.Logger {
	if t.log == nil {
		t.log = plog.NewDefaultLogger().WithPrefix("store " + t.Ref() + ": ")
	}
	return t.log
}

// tag returns the tag of the key and the value, the value escaped.
func tag(k, v string) string {
	return k + "=" + url.QueryEscape(v)
}

// backupArgs returns the arguments of plakar backup for a snapshot of the
// run, tagged as the run plus tags, named after the object for the snapshots
// to read in the Plakar interface without the tags.
func backupArgs(run backup.Run, tags ...string) []string {
	l := []string{
		tag(tagRun, run.ID),
		tag(tagObject, run.ObjectID),
		tag(tagPath, run.Path),
		tag(tagCluster, run.ClusterID),
		tag(tagClusterName, run.ClusterName),
		tag(tagNode, run.Node),
		tag(tagCreated, run.CreatedAt.UTC().Format(time.RFC3339Nano)),
	}
	if run.Protected {
		l = append(l, tagProtected)
	}
	if run.Note != "" {
		l = append(l, tag(tagNote, run.Note))
	}
	args := []string{"-tag", strings.Join(append(l, tags...), ",")}
	if run.Path != "" {
		args = append(args, "-name", run.Path)
	}
	return args
}

// ignorePattern returns the plakar -ignore pattern of an exclude. plakar
// anchors a pattern holding a slash at the root of the filesystem, not at
// the directory backed up: an anchored exclude is prefixed with src, its
// wildcards escaped. A leading ! or #, a negation or a comment to plakar, is
// escaped.
func ignorePattern(src, pattern string) string {
	pattern, anchored := backup.SplitPattern(pattern)
	if pattern == "" {
		return ""
	}
	if anchored {
		prefix := strings.TrimSuffix(backup.LiteralPattern(strings.TrimPrefix(filepath.Clean(src), "/")), "/")
		return prefix + "/" + pattern
	}
	if strings.HasPrefix(pattern, "!") || strings.HasPrefix(pattern, "#") {
		return `\` + pattern
	}
	return pattern
}

// within tells whether p is dir or under it.
func within(p, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(p))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// Backup saves the directory src as a snapshot, member of the run.
func (t *T) Backup(ctx context.Context, run backup.Run, member, src, head string, excludes []string) error {
	if err := t.ensure(ctx); err != nil {
		return err
	}
	if _, dir, _ := t.location(); dir != "" && within(dir, src) {
		return fmt.Errorf("store %s at %s is under %s: a store is never inside a head it backs up", t.Ref(), dir, src)
	}
	tags := []string{tag(tagMember, member), tag(tagKind, string(backup.MemberDir))}
	if head != "" {
		tags = append(tags, tag(tagHead, filepath.Clean(head)))
	}
	args := backupArgs(run, tags...)
	for _, pattern := range excludes {
		if s := ignorePattern(src, pattern); s != "" {
			args = append(args, "-ignore", s)
		}
	}
	_, err := t.run(ctx, zerolog.InfoLevel, "backup", append(args, src)...)
	return err
}

// BackupStream saves what r reads as the file /<member> of a snapshot,
// member of the run. A stream is read once: the call is not retried.
func (t *T) BackupStream(ctx context.Context, run backup.Run, member string, r io.Reader) error {
	if err := t.ensure(ctx); err != nil {
		return err
	}
	args := backupArgs(run, tag(tagMember, member), tag(tagKind, string(backup.MemberStream)))
	return t.backupStdin(ctx, r, append(args, "stdin:///"+member)...)
}

// Complete writes the marker of the run, an empty snapshot tagged complete,
// with the path of the object and the name of the cluster as they are now.
func (t *T) Complete(ctx context.Context, run backup.Run) error {
	if err := t.ensure(ctx); err != nil {
		return err
	}
	args := backupArgs(run, tagComplete, tag(tagCompleted, time.Now().UTC().Format(time.RFC3339Nano)))
	return t.backupStdin(ctx, strings.NewReader(""), append(args, "stdin:///"+markerName)...)
}

// backupStdin runs plakar backup reading r on its standard input.
func (t *T) backupStdin(ctx context.Context, r io.Reader, args ...string) error {
	cmd, err := t.commandAt(ctx, zerolog.InfoLevel, nil, "backup", args...)
	if err != nil {
		return err
	}
	cmd.Cmd().Stdin = r
	if err := cmd.Run(); err != nil {
		return cmdError("backup", cmd, err)
	}
	return nil
}

// snapshots returns the snapshots of the runs the store holds, oldest
// first. plakar filters on whole tags, never on a key whatever its value:
// the snapshots are listed all, and the ones of no run left out.
func (t *T) snapshots(ctx context.Context) ([]snapshot, error) {
	if err := t.ensure(ctx); err != nil {
		return nil, err
	}
	out, err := t.run(ctx, zerolog.TraceLevel, "ls", "-uuid", "-tags")
	if err != nil {
		return nil, err
	}
	all, err := parseLs(out)
	if err != nil {
		return nil, err
	}
	l := make([]snapshot, 0, len(all))
	// plakar lists the newest first.
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Tags[tagRun] != "" {
			l = append(l, all[i])
		}
	}
	return l, nil
}

// runsOf groups the snapshots, oldest first, by run, the members in the
// order they were saved.
func runsOf(snaps []snapshot) []backup.Run {
	byID := make(map[string]*backup.Run)
	order := make([]string, 0)
	for _, s := range snaps {
		id := s.Tags[tagRun]
		run, ok := byID[id]
		if !ok {
			_, protected := s.Tags[tagProtected]
			created, _ := time.Parse(time.RFC3339Nano, s.Tags[tagCreated])
			run = &backup.Run{
				ID:          id,
				ClusterID:   s.Tags[tagCluster],
				ClusterName: s.Tags[tagClusterName],
				Path:        s.Tags[tagPath],
				ObjectID:    s.Tags[tagObject],
				Node:        s.Tags[tagNode],
				CreatedAt:   created,
				Members:     make([]backup.Member, 0),
				Protected:   protected,
				Note:        s.Tags[tagNote],
			}
			byID[id] = run
			order = append(order, id)
		}
		if _, ok := s.Tags[tagComplete]; ok {
			run.CompletedAt, _ = time.Parse(time.RFC3339Nano, s.Tags[tagCompleted])
			continue
		}
		run.Members = append(run.Members, backup.Member{
			Name:         s.Tags[tagMember],
			Kind:         backup.MemberKind(s.Tags[tagKind]),
			Head:         s.Tags[tagHead],
			LogicalBytes: s.Size,
		})
		run.LogicalBytes += s.Size
	}
	l := make([]backup.Run, 0, len(order))
	for _, id := range order {
		l = append(l, *byID[id])
	}
	return l
}

// objectKey identifies the object of a run among the objects of every
// cluster.
func objectKey(run backup.Run) string {
	return run.ClusterID + "/" + run.ObjectID
}

// current returns, by object, the path the store last recorded, and by
// cluster id, the name: the ones of the latest completed run, or of the
// first run of an object none of whose runs completed.
func current(runs []backup.Run) (map[string]string, map[string]string) {
	completed := make([]backup.Run, 0, len(runs))
	for _, run := range runs {
		if !run.CompletedAt.IsZero() {
			completed = append(completed, run)
		}
	}
	slices.SortStableFunc(completed, func(a, b backup.Run) int { return a.CompletedAt.Compare(b.CompletedAt) })
	paths := make(map[string]string)
	names := make(map[string]string)
	for _, run := range completed {
		paths[objectKey(run)] = run.Path
		names[run.ClusterID] = run.ClusterName
	}
	for _, run := range runs {
		if _, ok := paths[objectKey(run)]; !ok {
			paths[objectKey(run)] = run.Path
		}
	}
	return paths, names
}

// runs returns the runs filter selects, oldest first, the cluster named as
// it is now, and the snapshots of the store.
func (t *T) runs(ctx context.Context, filter backup.RunFilter) ([]backup.Run, []snapshot, error) {
	snaps, err := t.snapshots(ctx)
	if err != nil {
		return nil, nil, err
	}
	all := runsOf(snaps)
	paths, names := current(all)
	l := make([]backup.Run, 0)
	for _, run := range all {
		if name, ok := names[run.ClusterID]; ok {
			run.ClusterName = name
		}
		if filter.Path != "" && paths[objectKey(run)] != filter.Path {
			continue
		}
		if !filter.Match(run) {
			continue
		}
		l = append(l, run)
	}
	slices.SortStableFunc(l, func(a, b backup.Run) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return l, snaps, nil
}

// Runs returns the runs filter selects, oldest first. Its errors name the
// store, which the listings spanning several stores print as they come.
func (t *T) Runs(ctx context.Context, filter backup.RunFilter) ([]backup.Run, error) {
	runs, _, err := t.runs(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("store %s: %w", t.Ref(), err)
	}
	return runs, nil
}

// ObjectStats counts the runs of every object of the store, of every
// cluster, from one listing: plakar keeps no record to read them from. Its
// errors name the store.
func (t *T) ObjectStats(ctx context.Context) ([]backup.ObjectStats, error) {
	snaps, err := t.snapshots(ctx)
	if err != nil {
		return nil, fmt.Errorf("store %s: %w", t.Ref(), err)
	}
	runs := runsOf(snaps)
	paths, names := current(runs)
	byObject := make(map[string][]backup.Run)
	order := make([]string, 0)
	for _, run := range runs {
		k := objectKey(run)
		if _, ok := byObject[k]; !ok {
			order = append(order, k)
		}
		byObject[k] = append(byObject[k], run)
	}
	l := make([]backup.ObjectStats, 0, len(order))
	for _, k := range order {
		run := byObject[k][0]
		l = append(l, backup.NewObjectStats(run.ClusterID, names[run.ClusterID], run.ObjectID, paths[k], byObject[k]))
	}
	return l, nil
}

// memberSnapshot returns the id of the snapshot holding the member of the
// run.
func (t *T) memberSnapshot(ctx context.Context, run backup.Run, member string) (string, error) {
	if err := t.ensure(ctx); err != nil {
		return "", err
	}
	out, err := t.run(ctx, zerolog.TraceLevel, "ls", "-uuid", "-tags", "-tag", tag(tagRun, run.ID)+","+tag(tagMember, member))
	if err != nil {
		return "", err
	}
	l, err := parseLs(out)
	if err != nil {
		return "", err
	}
	if len(l) == 0 {
		return "", fmt.Errorf("%w: store %s holds no member %s of the run %s", backup.ErrNotFound, t.Ref(), member, run.ShortID())
	}
	return l[0].ID, nil
}

// Restore writes the member of the run directly under dst: plakar strips
// the root the snapshot was made from, overwrites the files it restores and
// leaves the others.
func (t *T) Restore(ctx context.Context, run backup.Run, member string, dst string) error {
	id, err := t.memberSnapshot(ctx, run, member)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return err
	}
	_, err = t.run(ctx, zerolog.InfoLevel, "restore", "-to", dst, id)
	return err
}

// Cat returns the content of a member saved from a stream.
func (t *T) Cat(ctx context.Context, run backup.Run, member string) (io.ReadCloser, error) {
	if m, ok := run.Member(member); ok && m.Kind != backup.MemberStream {
		return nil, fmt.Errorf("member %s of the run %s was saved from a directory, not a stream", member, run.ShortID())
	}
	id, err := t.memberSnapshot(ctx, run, member)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	cmd, err := t.commandAt(ctx, zerolog.TraceLevel, &b, "cat", id+":/"+member)
	if err != nil {
		return nil, err
	}
	if err := cmd.Run(); err != nil {
		return nil, cmdError("cat", cmd, err)
	}
	return io.NopCloser(&b), nil
}

// Delete removes every snapshot of the runs, their markers too. A run with
// no snapshot, as a run failed before its first member, removes nothing.
func (t *T) Delete(ctx context.Context, runs []backup.Run) error {
	if err := t.ensure(ctx); err != nil {
		return err
	}
	for _, run := range runs {
		if _, err := t.run(ctx, zerolog.InfoLevel, "rm", "-apply", "-tag", tag(tagRun, run.ID)); err != nil {
			return err
		}
	}
	return nil
}

// Check verifies the snapshots of the runs filter selects, the whole store
// with a zero filter.
func (t *T) Check(ctx context.Context, filter backup.RunFilter) error {
	if filter == (backup.RunFilter{}) {
		if err := t.ensure(ctx); err != nil {
			return err
		}
		_, err := t.run(ctx, zerolog.TraceLevel, "check")
		return err
	}
	runs, snaps, err := t.runs(ctx, filter)
	if err != nil {
		return err
	}
	selected := make(map[string]bool)
	for _, run := range runs {
		selected[run.ID] = true
	}
	ids := make([]string, 0)
	for _, s := range snaps {
		if selected[s.Tags[tagRun]] {
			ids = append(ids, s.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	_, err = t.run(ctx, zerolog.TraceLevel, "check", ids...)
	return err
}
