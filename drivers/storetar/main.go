// Package storetar is the tar store driver: a directory, of the node or of an
// sftp server, holding, per run, a gzipped tar archive per head and a gzipped
// file per streamed member.
//
// The runs of an object are under <cluster id>/<object id>/<run id>: several
// clusters may write one store, an object keeps its runs when renamed, and an
// object created again at a path does not take the runs of the former one.
// An object.json beside the runs of an object says the path it had at its
// last completed run, which a listing by path reads, and a cluster.json beside
// the objects of a cluster the name it had then, which a listing by cluster
// name reads. Both are this driver's: another store records the same in its
// own way, as tags.
//
// It neither deduplicates nor encrypts. It is the reference store of the
// backup framework, exercising every capability with nothing but the
// standard library, and a store for tests.
package storetar

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/driver"
)

type (
	// T is the tar store driver.
	T struct {
		backup.T

		// fsys is the connection to the sftp server of the store, once
		// connected.
		fsys storeFS
	}

	// manifest is the description of a run, beside its members.
	manifest struct {
		backup.Run
	}

	// clusterMarker says, in the directory of a cluster, the name the
	// cluster had at its last completed run, which a listing shows and
	// selects the cluster by.
	clusterMarker struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	// marker says, in the directory of an object, the path the object had
	// at its last completed run: the runs are found by path through it, and
	// an object renamed keeps its runs.
	marker struct {
		Path        string    `json:"path"`
		ClusterName string    `json:"cluster_name"`
		UpdatedAt   time.Time `json:"updated_at"`

		// Stats are the figures of the runs of the object, recounted
		// from its directory as a run of it is started, completed or
		// removed, for the stats of the store to sum one record per
		// object.
		Stats *backup.ObjectStats `json:"stats,omitempty"`
	}

	// countWriter counts the bytes written through it.
	countWriter struct {
		w io.Writer
		n int64
	}
)

func (t *countWriter) Write(b []byte) (int, error) {
	n, err := t.w.Write(b)
	t.n += int64(n)
	return n, err
}

const (
	manifestName    = "manifest.json"
	markerName      = "object.json"
	clusterName     = "cluster.json"
	tmpSuffix       = ".tmp"
	sftpDialTimeout = 10 * time.Second
)

var (
	drvID = driver.NewID(driver.GroupStore, "tar")

	_ backup.Driver   = (*T)(nil)
	_ backup.Streamer = (*T)(nil)
	_ backup.Pruner   = (*T)(nil)
	_ backup.Localer  = (*T)(nil)
	_ backup.Sharer   = (*T)(nil)
	_ backup.Statser  = (*T)(nil)
	_ backup.Usager   = (*T)(nil)
)

func init() {
	driver.Register(drvID, New)
}

// New allocates a tar store driver.
func New() backup.Driver {
	return &T{}
}

// Close closes the connection to the sftp server of the store, if any.
func (t *T) Close() error {
	if t.fsys == nil {
		return nil
	}
	err := t.fsys.Close()
	t.fsys = nil
	return err
}

// objectDir returns the directory holding the runs of the object of the run:
// <cluster id>/<object id>, which a rename of the object does not change, and
// which another cluster writing the store, or an object of the same path,
// never shares.
func (t *T) objectDir(run backup.Run) (storeFS, string, error) {
	if err := validName("cluster id", run.ClusterID); err != nil {
		return nil, "", err
	}
	if err := validName("object id", run.ObjectID); err != nil {
		return nil, "", err
	}
	fsys, root, err := t.location()
	if err != nil {
		return nil, "", err
	}
	return fsys, filepath.Join(root, run.ClusterID, run.ObjectID), nil
}

// runDir returns the directory holding the run.
func (t *T) runDir(run backup.Run) (storeFS, string, error) {
	if err := validName("run id", run.ID); err != nil {
		return nil, "", err
	}
	fsys, dir, err := t.objectDir(run)
	if err != nil {
		return nil, "", err
	}
	return fsys, filepath.Join(dir, run.ID), nil
}

// validName refuses a name that is not one path element of its own, as a
// run id or a member name making a file name of the store.
func validName(what, s string) error {
	switch {
	case s == "", s == ".", s == "..", strings.ContainsAny(s, "/\\\x00"):
		return fmt.Errorf("invalid %s %q", what, s)
	}
	return nil
}

func memberFile(m backup.Member) string {
	if m.Kind == backup.MemberStream {
		return m.Name + ".gz"
	}
	return m.Name + ".tar.gz"
}

// Backup saves the directory src as a gzipped tar archive, member of the run.
func (t *T) Backup(ctx context.Context, run backup.Run, member, src, head string, excludes []string) error {
	if err := validName("member", member); err != nil {
		return err
	}
	fsys, root, err := t.location()
	if err != nil {
		return err
	}
	if _, ok := fsys.(localFS); ok && within(root, src) {
		return fmt.Errorf("store %s at %s is under %s: a store is never inside a head it backs up", t.Ref(), root, src)
	}
	m := backup.Member{Name: member, Kind: backup.MemberDir}
	if head != "" {
		m.Head = filepath.Clean(head)
	}
	return t.writeMember(ctx, run, m, func(w io.Writer) error {
		return writeTar(ctx, w, src, excludes)
	})
}

// BackupStream saves what r reads as a gzipped file, member of the run.
func (t *T) BackupStream(ctx context.Context, run backup.Run, member string, r io.Reader) error {
	if err := validName("member", member); err != nil {
		return err
	}
	m := backup.Member{Name: member, Kind: backup.MemberStream}
	return t.writeMember(ctx, run, m, func(w io.Writer) error {
		_, err := io.Copy(w, readerWithContext(ctx, r))
		return err
	})
}

// writeMember writes a member of the run through write, gzipped, then adds
// it to the manifest of the run. The member file is renamed in place once
// complete, and the manifest after it, so a run interrupted lists only the
// members it saved whole.
func (t *T) writeMember(ctx context.Context, run backup.Run, m backup.Member, write func(io.Writer) error) error {
	fsys, dir, err := t.runDir(run)
	if err != nil {
		return err
	}
	if err := fsys.MkdirAll(dir); err != nil {
		return err
	}
	final := filepath.Join(dir, memberFile(m))
	tmp := final + tmpSuffix
	f, err := fsys.Create(tmp)
	if err != nil {
		return err
	}
	defer func() { _ = fsys.Remove(tmp) }()
	stored := &countWriter{w: f}
	zw := gzip.NewWriter(stored)
	logical := &countWriter{w: zw}
	if err := write(logical); err != nil {
		_ = f.Close()
		return fmt.Errorf("member %s of run %s: %w", m.Name, run.ID, err)
	}
	if err := zw.Close(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := fsys.Rename(tmp, final); err != nil {
		return err
	}
	m.LogicalBytes, m.StoredBytes = logical.n, stored.n
	return t.addMember(fsys, dir, run, m)
}

// addMember records the member in the manifest of the run in dir.
// The first member of a run recounts the stats of the object, the run
// counted incomplete until it completes, so a run interrupted is counted.
func (t *T) addMember(fsys storeFS, dir string, run backup.Run, m backup.Member) error {
	mf, err := readManifest(fsys, dir)
	first := false
	switch {
	case errors.Is(err, os.ErrNotExist):
		mf = manifest{Run: run}
		mf.Members = nil
		first = true
	case err != nil:
		return err
	}
	if i := slices.IndexFunc(mf.Members, func(e backup.Member) bool { return e.Name == m.Name }); i >= 0 {
		mf.Members[i] = m
	} else {
		mf.Members = append(mf.Members, m)
	}
	mf.LogicalBytes, mf.StoredBytes = 0, 0
	for _, e := range mf.Members {
		mf.LogicalBytes += e.LogicalBytes
		mf.StoredBytes += e.StoredBytes
	}
	if err := writeManifest(fsys, dir, mf); err != nil {
		return err
	}
	if first {
		return t.recountObject(fsys, filepath.Dir(dir), run, false)
	}
	return nil
}

// Complete marks the run completed in its manifest.
func (t *T) Complete(ctx context.Context, run backup.Run) error {
	fsys, dir, err := t.runDir(run)
	if err != nil {
		return err
	}
	mf, err := readManifest(fsys, dir)
	if err != nil {
		return err
	}
	mf.CompletedAt = time.Now()
	if err := writeManifest(fsys, dir, mf); err != nil {
		return err
	}
	objectDir := filepath.Dir(dir)
	if err := t.recountObject(fsys, objectDir, run, true); err != nil {
		return err
	}
	return writeJSON(fsys, filepath.Join(filepath.Dir(objectDir), clusterName), clusterMarker{ID: run.ClusterID, Name: run.ClusterName, UpdatedAt: mf.CompletedAt})
}

// recountObject writes the marker of the object in objectDir with the stats
// of its runs, read from its directory. A completed run of it names the path
// and the cluster the marker says, the path the runs are found by. Otherwise
// the marker keeps what it says, or says the run's when it is the object's
// first.
func (t *T) recountObject(fsys storeFS, objectDir string, run backup.Run, completed bool) error {
	runs, err := objectRuns(fsys, objectDir)
	if err != nil {
		return err
	}
	m, err := readMarker(fsys, objectDir)
	switch {
	case completed || errors.Is(err, os.ErrNotExist):
		m.Path, m.ClusterName, m.UpdatedAt = run.Path, run.ClusterName, time.Now()
	case err != nil:
		return err
	}
	stats := backup.NewObjectStats(filepath.Base(filepath.Dir(objectDir)), m.ClusterName, filepath.Base(objectDir), m.Path, runs)
	m.Stats = &stats
	return writeJSON(fsys, filepath.Join(objectDir, markerName), m)
}

// objectRuns returns the runs of the object in objectDir, the incomplete
// ones too.
func objectRuns(fsys storeFS, objectDir string) ([]backup.Run, error) {
	matches, err := fsys.Glob(filepath.Join(objectDir, "*", manifestName))
	if err != nil {
		return nil, err
	}
	runs := make([]backup.Run, 0, len(matches))
	for _, match := range matches {
		mf, err := readManifest(fsys, filepath.Dir(match))
		if errors.Is(err, os.ErrNotExist) {
			// removed meanwhile
			continue
		} else if err != nil {
			return nil, err
		}
		runs = append(runs, mf.Run)
	}
	return runs, nil
}

// ObjectStats returns the stats every object of the store records, of every
// cluster, the cluster named as it is now. An object whose marker holds none,
// written before the stats were, is recounted from its directory, without
// writing it.
func (t *T) ObjectStats(ctx context.Context) ([]backup.ObjectStats, error) {
	fsys, root, err := t.location()
	if err != nil {
		return nil, err
	}
	if err := fsys.Stat(root); errors.Is(err, os.ErrNotExist) && t.IsLocal() {
		return []backup.ObjectStats{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("store %s: %w", t.Ref(), err)
	}
	matches, err := fsys.Glob(filepath.Join(root, "*", "*", markerName))
	if err != nil {
		return nil, fmt.Errorf("store %s: %w", t.Ref(), err)
	}
	names := make(map[string]string)
	l := make([]backup.ObjectStats, 0, len(matches))
	for _, match := range matches {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		objectDir := filepath.Dir(match)
		clusterDir := filepath.Dir(objectDir)
		clusterID := filepath.Base(clusterDir)
		if _, ok := names[clusterID]; !ok {
			if cm, err := readClusterMarker(fsys, clusterDir); err == nil {
				names[clusterID] = cm.Name
			} else {
				names[clusterID] = ""
			}
		}
		m, err := readMarker(fsys, objectDir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("store %s: %w", t.Ref(), err)
		}
		var stats backup.ObjectStats
		if m.Stats != nil {
			stats = *m.Stats
		} else {
			runs, err := objectRuns(fsys, objectDir)
			if err != nil {
				return nil, fmt.Errorf("store %s: %w", t.Ref(), err)
			}
			stats = backup.NewObjectStats(clusterID, m.ClusterName, filepath.Base(objectDir), m.Path, runs)
		}
		if name := names[clusterID]; name != "" {
			stats.ClusterName = name
		}
		l = append(l, stats)
	}
	return l, nil
}

// Usage says the space left where the store keeps its runs. What they take
// is summed from the stats of the objects.
func (t *T) Usage(ctx context.Context) (backup.Usage, error) {
	fsys, root, err := t.location()
	if err != nil {
		return backup.Usage{}, err
	}
	dir := root
	if err := fsys.Stat(dir); err != nil {
		// A node writes its directory with its first run: the space is
		// the one of where it will be.
		dir = filepath.Dir(root)
	}
	free, ok, err := fsys.Free(dir)
	if err != nil || !ok {
		return backup.Usage{}, err
	}
	return backup.Usage{FreeBytes: &free}, nil
}

// writeJSON writes v as the file final, in place once complete.
func writeJSON(fsys storeFS, final string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := final + tmpSuffix
	if err := fsys.WriteFile(tmp, b); err != nil {
		return err
	}
	return fsys.Rename(tmp, final)
}

func readClusterMarker(fsys storeFS, dir string) (clusterMarker, error) {
	var m clusterMarker
	b, err := fsys.ReadFile(filepath.Join(dir, clusterName))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("%s: %w", filepath.Join(dir, clusterName), err)
	}
	return m, nil
}

func readMarker(fsys storeFS, dir string) (marker, error) {
	var m marker
	b, err := fsys.ReadFile(filepath.Join(dir, markerName))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("%s: %w", filepath.Join(dir, markerName), err)
	}
	return m, nil
}

// writeManifest writes the manifest of the run in dir, in place once
// complete.
func writeManifest(fsys storeFS, dir string, mf manifest) error {
	b, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(dir, manifestName)
	tmp := final + tmpSuffix
	if err := fsys.WriteFile(tmp, b); err != nil {
		return err
	}
	return fsys.Rename(tmp, final)
}

func readManifest(fsys storeFS, dir string) (manifest, error) {
	var mf manifest
	b, err := fsys.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return mf, err
	}
	if err := json.Unmarshal(b, &mf); err != nil {
		return mf, fmt.Errorf("%s: %w", filepath.Join(dir, manifestName), err)
	}
	return mf, nil
}

// Runs returns the runs filter selects, oldest first: the object directories
// its cluster and object ids name, or every one, and of them the ones whose
// marker says its path. Its errors name the store, which the listings
// spanning several stores print as they come.
func (t *T) Runs(ctx context.Context, filter backup.RunFilter) ([]backup.Run, error) {
	fsys, root, err := t.location()
	if err != nil {
		return nil, err
	}
	if err := fsys.Stat(root); errors.Is(err, os.ErrNotExist) && t.IsLocal() {
		// A node writes its directory with its first run.
		return []backup.Run{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("store %s: %w", t.Ref(), err)
	}
	clusterDirs, err := t.clusterDirs(fsys, root, filter.Cluster)
	if err != nil {
		return nil, fmt.Errorf("store %s: %w", t.Ref(), err)
	}
	objectGlob := "*"
	if filter.ObjectID != "" {
		if err := validName("object id", filter.ObjectID); err != nil {
			return nil, fmt.Errorf("store %s: %w", t.Ref(), err)
		}
		objectGlob = filter.ObjectID
	}
	// The cluster is selected by its directory: a run written before the
	// cluster was renamed says the former name.
	filter.Cluster = ""
	names := make(map[string]string)
	dirs := make([]string, 0)
	for _, clusterDir := range clusterDirs {
		if m, err := readClusterMarker(fsys, clusterDir); err == nil {
			names[filepath.Base(clusterDir)] = m.Name
		}
		l, err := fsys.Glob(filepath.Join(clusterDir, objectGlob))
		if err != nil {
			return nil, fmt.Errorf("store %s: %w", t.Ref(), err)
		}
		for _, dir := range l {
			if filepath.Base(dir) != clusterName {
				dirs = append(dirs, dir)
			}
		}
	}
	runs := make([]backup.Run, 0)
	var errs error
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if filter.Path != "" {
			m, err := readMarker(fsys, dir)
			switch {
			case errors.Is(err, os.ErrNotExist):
				// No run of the object completed.
				continue
			case err != nil:
				errs = errors.Join(errs, err)
				continue
			case m.Path != filter.Path:
				continue
			}
		}
		matches, err := fsys.Glob(filepath.Join(dir, "*", manifestName))
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		for _, match := range matches {
			mf, err := readManifest(fsys, filepath.Dir(match))
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}
			if filepath.Base(filepath.Dir(match)) != mf.ID {
				errs = errors.Join(errs, fmt.Errorf("%s: run %s in the directory of run %s", match, mf.ID, filepath.Base(filepath.Dir(match))))
				continue
			}
			if !filter.Match(mf.Run) {
				continue
			}
			if name, ok := names[mf.ClusterID]; ok {
				// The cluster as it is named now.
				mf.ClusterName = name
			}
			runs = append(runs, mf.Run)
		}
	}
	slices.SortStableFunc(runs, func(a, b backup.Run) int { return a.CreatedAt.Compare(b.CreatedAt) })
	if errs != nil {
		errs = fmt.Errorf("store %s: %w", t.Ref(), errs)
	}
	return runs, errs
}

// clusterDirs returns the directories of the clusters the filter names, by id
// or by the name their cluster.json says, of every cluster when it names
// none. The directories holding files are left to the glob of their objects.
func (t *T) clusterDirs(fsys storeFS, root, cluster string) ([]string, error) {
	if cluster == "" {
		return fsys.Glob(filepath.Join(root, "*"))
	}
	if err := validName("cluster", cluster); err != nil {
		return nil, err
	}
	if fsys.Stat(filepath.Join(root, cluster)) == nil {
		return []string{filepath.Join(root, cluster)}, nil
	}
	markers, err := fsys.Glob(filepath.Join(root, "*", clusterName))
	if err != nil {
		return nil, err
	}
	l := make([]string, 0)
	for _, p := range markers {
		if m, err := readClusterMarker(fsys, filepath.Dir(p)); err == nil && m.Name == cluster {
			l = append(l, filepath.Dir(p))
		}
	}
	return l, nil
}

// openMember opens the gzipped file of the member of the run.
func (t *T) openMember(run backup.Run, member string) (backup.Member, io.ReadCloser, error) {
	m, ok := run.Member(member)
	if !ok {
		return m, nil, fmt.Errorf("run %s of %s has no member %s", run.ID, run.Path, member)
	}
	if err := validName("member", member); err != nil {
		return m, nil, err
	}
	fsys, dir, err := t.runDir(run)
	if err != nil {
		return m, nil, err
	}
	f, err := fsys.OpenIn(dir, memberFile(m))
	if err != nil {
		return m, nil, err
	}
	zr, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		return m, nil, fmt.Errorf("member %s of run %s: %w", member, run.ID, err)
	}
	return m, readCloser{Reader: zr, closers: []io.Closer{zr, f}}, nil
}

// Restore writes the member of the run into dst: the tree of a head, or the
// file of a streamed member, named after it.
func (t *T) Restore(ctx context.Context, run backup.Run, member string, dst string) error {
	m, rc, err := t.openMember(run, member)
	if err != nil {
		return err
	}
	defer rc.Close()
	if err := os.MkdirAll(dst, 0700); err != nil {
		return err
	}
	dstRoot, err := os.OpenRoot(dst)
	if err != nil {
		return err
	}
	defer dstRoot.Close()
	if m.Kind == backup.MemberStream {
		f, err := dstRoot.OpenFile(member, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, readerWithContext(ctx, rc)); err != nil {
			_ = f.Close()
			return err
		}
		return f.Close()
	}
	return extractTar(ctx, rc, dstRoot)
}

// Cat returns a reader of the member of the run: the document of a streamed
// member, the tar archive of a head.
func (t *T) Cat(ctx context.Context, run backup.Run, member string) (io.ReadCloser, error) {
	_, rc, err := t.openMember(run, member)
	return rc, err
}

// Delete removes the runs.
func (t *T) Delete(ctx context.Context, runs []backup.Run) error {
	var errs error
	for _, run := range runs {
		if err := ctx.Err(); err != nil {
			return err
		}
		fsys, dir, err := t.runDir(run)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		if err := fsys.RemoveAll(dir); err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		if err := t.recountObject(fsys, filepath.Dir(dir), run, false); err != nil {
			errs = errors.Join(errs, fmt.Errorf("recount the stats of %s: %w", run.Path, err))
		}
	}
	return errs
}

// Check reads every member of the runs filter selects to its end, which the
// gzip checksums verify.
func (t *T) Check(ctx context.Context, filter backup.RunFilter) error {
	runs, err := t.Runs(ctx, filter)
	if err != nil {
		return err
	}
	var errs error
	for _, run := range runs {
		for _, m := range run.Members {
			if err := t.checkMember(ctx, run, m); err != nil {
				errs = errors.Join(errs, fmt.Errorf("run %s of %s, member %s: %w", run.ID, run.Path, m.Name, err))
			}
		}
	}
	return errs
}

func (t *T) checkMember(ctx context.Context, run backup.Run, m backup.Member) error {
	_, rc, err := t.openMember(run, m.Name)
	if err != nil {
		return err
	}
	defer rc.Close()
	r := readerWithContext(ctx, rc)
	if m.Kind == backup.MemberStream {
		_, err := io.Copy(io.Discard, r)
		return err
	}
	tr := tar.NewReader(r)
	for {
		_, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return err
		}
	}
}

// within tells whether p is dir or under it.
func within(p, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(p))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// writeTar writes the tree under src as a tar archive, its paths relative to
// src, without the paths excludes matches.
func writeTar(ctx context.Context, w io.Writer, src string, excludes []string) error {
	// The files are opened through the root, which no symlink escapes:
	// an entry replaced by one after the walk saw it, by a process
	// writing the head, is not read from out of the head.
	root, err := os.OpenRoot(src)
	if err != nil {
		return err
	}
	defer root.Close()
	tw := tar.NewWriter(w)
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel != "." && backup.Excluded(rel, excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		var link string
		if info.Mode()&fs.ModeSymlink != 0 {
			if link, err = os.Readlink(p); err != nil {
				return err
			}
		}
		switch {
		case info.Mode().IsRegular(), info.IsDir(), info.Mode()&fs.ModeSymlink != 0:
		default:
			// Sockets, fifos and devices hold no data to restore.
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = rel
		if info.IsDir() {
			hdr.Name += "/"
		}
		hdr.Uname, hdr.Gname = "", ""
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := root.Open(filepath.FromSlash(rel))
		if err != nil {
			return err
		}
		defer f.Close()
		if fi, err := f.Stat(); err != nil {
			return err
		} else if !os.SameFile(info, fi) {
			return fmt.Errorf("%s: replaced while saved", p)
		}
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return err
	}
	return tw.Close()
}

// extractTar writes the tree of the tar archive r reads into the directory
// root opens, which no entry escapes: the modes, owners and times of the
// archive applied, the times of the directories last, once filled.
func extractTar(ctx context.Context, r io.Reader, root *os.Root) error {
	type dirTime struct {
		name string
		tm   time.Time
	}
	var dirTimes []dirTime
	tr := tar.NewReader(r)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		name := path.Clean(strings.TrimPrefix(hdr.Name, "/"))
		if name == "." {
			if err := applyMeta(root, ".", hdr, false); err != nil {
				return err
			}
			dirTimes = append(dirTimes, dirTime{".", hdr.ModTime})
			continue
		}
		if dir := path.Dir(name); dir != "." {
			if err := root.MkdirAll(dir, 0755); err != nil {
				return err
			}
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0700); err != nil {
				return err
			}
			if err := applyMeta(root, name, hdr, false); err != nil {
				return err
			}
			dirTimes = append(dirTimes, dirTime{name, hdr.ModTime})
		case tar.TypeReg:
			if err := removeIfNotRegular(root, name); err != nil {
				return err
			}
			f, err := root.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			if err := applyMeta(root, name, hdr, false); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := root.RemoveAll(name); err != nil {
				return err
			}
			if err := root.Symlink(hdr.Linkname, name); err != nil {
				return err
			}
			if err := applyMeta(root, name, hdr, true); err != nil {
				return err
			}
		case tar.TypeLink:
			target := path.Clean(strings.TrimPrefix(hdr.Linkname, "/"))
			if err := root.RemoveAll(name); err != nil {
				return err
			}
			if err := root.Link(target, name); err != nil {
				return err
			}
		}
	}
	for i := len(dirTimes) - 1; i >= 0; i-- {
		if err := root.Chtimes(dirTimes[i].name, dirTimes[i].tm, dirTimes[i].tm); err != nil {
			return err
		}
	}
	return nil
}

// removeIfNotRegular removes what name is when it is not a regular file, as
// a directory or a symlink a regular file of the archive replaces: writing
// through a symlink would write where it points.
func removeIfNotRegular(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return err
	case info.Mode().IsRegular():
		return nil
	default:
		return root.RemoveAll(name)
	}
}

// applyMeta gives name the owner, mode and time of the archive entry: a
// symlink its owner only.
func applyMeta(root *os.Root, name string, hdr *tar.Header, isLink bool) error {
	if err := root.Lchown(name, hdr.Uid, hdr.Gid); err != nil && !errors.Is(err, os.ErrPermission) {
		return err
	}
	if isLink {
		return nil
	}
	if err := root.Chmod(name, fs.FileMode(hdr.Mode).Perm()|modeBits(hdr.Mode)); err != nil {
		return err
	}
	return root.Chtimes(name, hdr.ModTime, hdr.ModTime)
}

// modeBits returns the setuid, setgid and sticky bits of a tar mode as file
// mode bits.
func modeBits(mode int64) fs.FileMode {
	var m fs.FileMode
	if mode&04000 != 0 {
		m |= fs.ModeSetuid
	}
	if mode&02000 != 0 {
		m |= fs.ModeSetgid
	}
	if mode&01000 != 0 {
		m |= fs.ModeSticky
	}
	return m
}

type readCloser struct {
	io.Reader
	closers []io.Closer
}

func (t readCloser) Close() error {
	var errs error
	for _, c := range t.closers {
		errs = errors.Join(errs, c.Close())
	}
	return errs
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (t ctxReader) Read(b []byte) (int, error) {
	if err := t.ctx.Err(); err != nil {
		return 0, err
	}
	return t.r.Read(b)
}

func readerWithContext(ctx context.Context, r io.Reader) io.Reader {
	return ctxReader{ctx: ctx, r: r}
}
