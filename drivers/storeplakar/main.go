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
// with the passphrase_cmd of @store running om <sec> key decode -- <key>:
// the passphrase goes through a pipe only, never in a file, the command line
// or the environment.
//
// A remote store reaches its location with the credentials its keywords
// name, keys of a sec, never with what the node holds for the user om runs
// as: see credentials.
//
// plakar derives the key of the store from the passphrase at each call, a
// second of work and 256 MiB: the driver lists the snapshots once and serves
// the lookups that follow from that listing, until it writes. Only a call
// writing a run creates a store of the node missing: a read of it finds no
// run.
//
// plakar unpacks its packages into a cache directory not holding them, and
// prints a line per file on its standard output: the first call of a driver
// is a plakar version, so no listing nor member is read with these lines.
//
// plakar rm unlinks snapshots and frees nothing: plakar maintenance does, in
// two passes, marking the packs no snapshot uses then removing them once
// their grace period is over. The driver runs it after a deletion.
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
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// T is the plakar store driver.
	T struct {
		backup.T
		log *plog.Logger

		// listings are the plakar ls outputs read since the last write of
		// this driver, by the tags they were narrowed with, the whole
		// listing under the empty key. A driver serves one command or one
		// action, so what another process writes meanwhile is seen by the
		// next one.
		listings   map[string][]snapshot
		listingsMu sync.Mutex

		// warmed says plakar readied its cache directory in a call of
		// this driver, whose output was dropped.
		warmed bool
		warmMu sync.Mutex
	}

	// tagValue is a tag a listing is narrowed with, its value not escaped.
	tagValue struct {
		key, value string
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

	// rmNoMatchMessage is what plakar rm says of a selection matching no
	// snapshot, exiting 0.
	rmNoMatchMessage = "no snapshots matched"

	// lockedMessage is what plakar maintenance says when another plakar
	// holds a lock on the store, as a backup running, whatever it exits
	// with.
	lockedMessage = "Can't take exclusive lock"
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
	// The store is judged before it is created: a store refused is not
	// left behind in the head.
	_, dir, err := t.location()
	if err != nil {
		return err
	}
	if dir != "" && within(dir, src) {
		return fmt.Errorf("store %s at %s is under %s: a store is never inside a head it backs up", t.Ref(), dir, src)
	}
	if err := t.ensure(ctx); err != nil {
		return err
	}
	defer t.forget()
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
	_, err = t.run(ctx, zerolog.InfoLevel, "backup", append(args, src)...)
	return err
}

// BackupStream saves what r reads as the file /<member> of a snapshot,
// member of the run. A stream is read once: the call is not retried.
func (t *T) BackupStream(ctx context.Context, run backup.Run, member string, r io.Reader) error {
	if err := t.ensure(ctx); err != nil {
		return err
	}
	defer t.forget()
	args := backupArgs(run, tag(tagMember, member), tag(tagKind, string(backup.MemberStream)))
	return t.backupStdin(ctx, r, append(args, "stdin:///"+member)...)
}

// Complete writes the marker of the run, an empty snapshot tagged complete,
// with the path of the object and the name of the cluster as they are now.
func (t *T) Complete(ctx context.Context, run backup.Run) error {
	if err := t.ensure(ctx); err != nil {
		return err
	}
	defer t.forget()
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

// String is the tag as plakar holds it, the value escaped.
func (t tagValue) String() string {
	return tag(t.key, t.value)
}

// holds tells whether the snapshot holds every tag.
func (s snapshot) holds(tags []tagValue) bool {
	for _, tv := range tags {
		if v, ok := s.Tags[tv.key]; !ok || v != tv.value {
			return false
		}
	}
	return true
}

// forget drops the listings read, which a write of the driver outdates.
func (t *T) forget() {
	t.listingsMu.Lock()
	defer t.listingsMu.Unlock()
	t.listings = nil
}

// Close drops the listings read.
func (t *T) Close() error {
	t.forget()
	return t.T.Close()
}

// list returns the snapshots holding every tag, the newest first as plakar
// lists them: read from the store once, then served from the listing read,
// or from the whole listing when it was. plakar selects the snapshots
// holding all the tags of a comma-separated -tag. A store of the node not
// created yet holds none.
func (t *T) list(ctx context.Context, tags ...tagValue) ([]snapshot, error) {
	names := make([]string, len(tags))
	for i, tv := range tags {
		names[i] = tv.String()
	}
	key := strings.Join(names, ",")
	t.listingsMu.Lock()
	defer t.listingsMu.Unlock()
	if l, ok := t.listings[key]; ok {
		return l, nil
	}
	if all, ok := t.listings[""]; ok {
		l := make([]snapshot, 0)
		for _, s := range all {
			if s.holds(tags) {
				l = append(l, s)
			}
		}
		return l, nil
	}
	l := make([]snapshot, 0)
	if ok, err := t.open(); err != nil {
		return nil, err
	} else if ok {
		args := []string{"-uuid", "-tags"}
		if key != "" {
			args = append(args, "-tag", key)
		}
		out, err := t.run(ctx, zerolog.TraceLevel, "ls", args...)
		if err != nil {
			return nil, err
		}
		if l, err = parseLs(out); err != nil {
			return nil, err
		}
	}
	if t.listings == nil {
		t.listings = make(map[string][]snapshot)
	}
	t.listings[key] = l
	return l, nil
}

// snapshots returns the snapshots of the runs the store holds, the ones
// holding every tag, oldest first. plakar filters on whole tags, never on a
// key whatever its value: the snapshots of no run are left out here.
func (t *T) snapshots(ctx context.Context, tags ...tagValue) ([]snapshot, error) {
	all, err := t.list(ctx, tags...)
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

// narrow returns the tags the listing of the runs filter selects is narrowed
// with: the ones of the object and of its cluster when filter names both,
// the cluster by its id, a name being the one of a marker, not a tag every
// snapshot holds. Such a listing holds every run of the object, so the path
// the object is listed by is the one the whole listing says.
func narrow(filter backup.RunFilter) []tagValue {
	if filter.ObjectID == "" || filter.Cluster == "" {
		return nil
	}
	if _, err := uuid.Parse(filter.Cluster); err != nil {
		return nil
	}
	return []tagValue{{tagCluster, filter.Cluster}, {tagObject, filter.ObjectID}}
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
// it is now, and the snapshots read.
//
// A filter naming an object and its cluster by id reads their snapshots
// only: the cluster is then named as the latest completed run of the object
// says, which lags a rename of the cluster until the next run of the object.
// Such a listing found empty is read again whole, for a cluster named as an
// id is.
func (t *T) runs(ctx context.Context, filter backup.RunFilter) ([]backup.Run, []snapshot, error) {
	tags := narrow(filter)
	snaps, err := t.snapshots(ctx, tags...)
	if err == nil && len(tags) > 0 && len(snaps) == 0 {
		snaps, err = t.snapshots(ctx)
	}
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
// run: from a listing already read when one holds it, as the one the run was
// found in, else from the listing of the snapshots of the run, read once for
// all its members.
func (t *T) memberSnapshot(ctx context.Context, run backup.Run, member string) (string, error) {
	tags := []tagValue{{tagRun, run.ID}, {tagMember, member}}
	t.listingsMu.Lock()
	for _, l := range t.listings {
		for _, s := range l {
			if s.holds(tags) {
				t.listingsMu.Unlock()
				return s.ID, nil
			}
		}
	}
	t.listingsMu.Unlock()
	l, err := t.list(ctx, tagValue{tagRun, run.ID})
	if err != nil {
		return "", err
	}
	for _, s := range l {
		if s.holds(tags) {
			return s.ID, nil
		}
	}
	return "", fmt.Errorf("%w: store %s holds no member %s of the run %s", backup.ErrNotFound, t.Ref(), member, run.ShortID())
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

// Delete removes every snapshot of the runs, their markers too, then has
// plakar reclaim their space. A run with no snapshot, as a run failed before
// its first member, removes nothing, and a store of the node not created
// yet holds no run to remove: a deletion never creates a store.
func (t *T) Delete(ctx context.Context, runs []backup.Run) error {
	if ok, err := t.open(); err != nil || !ok {
		return err
	}
	defer t.forget()
	removed := false
	for _, run := range runs {
		out, err := t.run(ctx, zerolog.InfoLevel, "rm", "-apply", "-tag", tag(tagRun, run.ID))
		if err != nil {
			return err
		}
		if !bytes.Contains(out, []byte(rmNoMatchMessage)) {
			removed = true
		}
	}
	if !removed {
		return nil
	}
	return t.maintain(ctx)
}

// maintain runs plakar maintenance, which frees the space of the snapshots
// removed: rm only unlinks them. plakar marks the packs no snapshot uses on
// a first pass and removes them on a pass run once their grace period, 7
// days, is over, so the space of a run is freed by the maintenance of a
// later deletion.
//
// The maintenance needs the store for itself. Another plakar holding a lock,
// as the backup of another object, makes it give up, saying so on its
// output, with an exit code that is not always an error: this is no failure
// of the deletion, the next one reclaims the space.
func (t *T) maintain(ctx context.Context) error {
	cmd, err := t.commandAt(ctx, zerolog.InfoLevel, nil, "maintenance")
	if err != nil {
		return err
	}
	err = cmd.Run()
	if bytes.Contains(cmd.Stdout(), []byte(lockedMessage)) || bytes.Contains(cmd.Stderr(), []byte(lockedMessage)) {
		t.Log().Infof("maintenance skipped: repository busy")
		return nil
	}
	if err != nil {
		return fmt.Errorf("the runs are deleted, their space is not reclaimed: %w", cmdError("maintenance", cmd, err))
	}
	return nil
}

// Check verifies the snapshots of the runs filter selects, the whole store
// with a zero filter. A store of the node not created yet holds no run to
// verify, and is refused when the whole of it is to check.
func (t *T) Check(ctx context.Context, filter backup.RunFilter) error {
	if filter == (backup.RunFilter{}) {
		if ok, err := t.open(); err != nil {
			return err
		} else if !ok {
			// A check of nothing does not pass: the store may be a
			// shared directory not mounted on this node.
			return fmt.Errorf("store %s: no store at %s to check", t.Ref(), t.Get("url"))
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
