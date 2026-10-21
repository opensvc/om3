// Package storetar is the tar store driver: a directory holding, per run, a
// gzipped tar archive per head and a gzipped file per streamed member.
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
	"github.com/opensvc/om3/v3/core/naming"
)

type (
	// T is the tar store driver.
	T struct {
		backup.T
	}

	// manifest is the description of a run, beside its members.
	manifest struct {
		backup.Run
	}
)

const (
	manifestName = "manifest.json"
	tmpSuffix    = ".tmp"
)

var (
	drvID = driver.NewID(driver.GroupStore, "tar")

	_ backup.Driver   = (*T)(nil)
	_ backup.Streamer = (*T)(nil)
	_ backup.Pruner   = (*T)(nil)
)

func init() {
	driver.Register(drvID, New)
}

// New allocates a tar store driver.
func New() backup.Driver {
	return &T{}
}

// root returns the directory holding the store, from its url.
func (t *T) root() (string, error) {
	s := t.Get("url")
	s = strings.TrimPrefix(s, "file://")
	switch {
	case s == "":
		return "", fmt.Errorf("store %s: no url", t.Ref())
	case !filepath.IsAbs(s):
		return "", fmt.Errorf("store %s: url %q is not an absolute path", t.Ref(), s)
	}
	return filepath.Clean(s), nil
}

// objectDir returns the directory holding the runs of the object at p.
func (t *T) objectDir(p string) (string, error) {
	root, err := t.root()
	if err != nil {
		return "", err
	}
	objPath, err := naming.ParsePath(p)
	if err != nil {
		return "", fmt.Errorf("run of %q: %w", p, err)
	}
	return filepath.Join(root, objPath.Namespace, objPath.Kind.String(), objPath.Name), nil
}

// runDir returns the directory holding the run.
func (t *T) runDir(run backup.Run) (string, error) {
	if err := validName("run id", run.ID); err != nil {
		return "", err
	}
	dir, err := t.objectDir(run.Path)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, run.ID), nil
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
func (t *T) Backup(ctx context.Context, run backup.Run, member string, src string, excludes []string) error {
	if err := validName("member", member); err != nil {
		return err
	}
	root, err := t.root()
	if err != nil {
		return err
	}
	if within(root, src) {
		return fmt.Errorf("store %s at %s is under %s: a store is never inside a head it backs up", t.Ref(), root, src)
	}
	m := backup.Member{Name: member, Kind: backup.MemberDir}
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
	dir, err := t.runDir(run)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	final := filepath.Join(dir, memberFile(m))
	tmp := final + tmpSuffix
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	zw := gzip.NewWriter(f)
	if err := write(zw); err != nil {
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
	if err := os.Rename(tmp, final); err != nil {
		return err
	}
	return t.addMember(dir, run, m)
}

// addMember records the member in the manifest of the run in dir.
func (t *T) addMember(dir string, run backup.Run, m backup.Member) error {
	mf, err := readManifest(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		mf = manifest{Run: run}
		mf.Members = nil
	case err != nil:
		return err
	}
	if i := slices.IndexFunc(mf.Members, func(e backup.Member) bool { return e.Name == m.Name }); i >= 0 {
		mf.Members[i] = m
	} else {
		mf.Members = append(mf.Members, m)
	}
	return writeManifest(dir, mf)
}

// Complete marks the run completed in its manifest.
func (t *T) Complete(ctx context.Context, run backup.Run) error {
	dir, err := t.runDir(run)
	if err != nil {
		return err
	}
	mf, err := readManifest(dir)
	if err != nil {
		return err
	}
	mf.CompletedAt = time.Now()
	return writeManifest(dir, mf)
}

// writeManifest writes the manifest of the run in dir, in place once
// complete.
func writeManifest(dir string, mf manifest) error {
	b, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(dir, manifestName)
	tmp := final + tmpSuffix
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

func readManifest(dir string) (manifest, error) {
	var mf manifest
	b, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return mf, err
	}
	if err := json.Unmarshal(b, &mf); err != nil {
		return mf, fmt.Errorf("%s: %w", filepath.Join(dir, manifestName), err)
	}
	return mf, nil
}

// Runs returns the runs filter selects, oldest first.
func (t *T) Runs(ctx context.Context, filter backup.RunFilter) ([]backup.Run, error) {
	root, err := t.root()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("store %s: %w", t.Ref(), err)
	}
	pattern := filepath.Join(root, "*", "*", "*", "*", manifestName)
	if filter.Path != "" {
		dir, err := t.objectDir(filter.Path)
		if err != nil {
			return nil, err
		}
		pattern = filepath.Join(dir, "*", manifestName)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	runs := make([]backup.Run, 0, len(matches))
	var errs error
	for _, match := range matches {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		mf, err := readManifest(filepath.Dir(match))
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
		runs = append(runs, mf.Run)
	}
	slices.SortStableFunc(runs, func(a, b backup.Run) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return runs, errs
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
	dir, err := t.runDir(run)
	if err != nil {
		return m, nil, err
	}
	f, err := os.Open(filepath.Join(dir, memberFile(m)))
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
		dir, err := t.runDir(run)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			errs = errors.Join(errs, err)
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
	tw := tar.NewWriter(w)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
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
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
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
