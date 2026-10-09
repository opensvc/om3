// Package fssnap takes read-only, point in time snapshots of the filesystems
// holding a set of paths, for a copy to read a stable source rather than one
// written to while it is read, and removes them after the copy, the
// snapshots a run killed before its end included.
//
// A path is snapshotted with the filesystem mounted deepest under it:
//
//   - a logical volume, thick or thin, whose snapshot is mounted read-only,
//   - a btrfs subvolume, whose read-only snapshot is reached through a
//     private mount of the filesystem root,
//   - a zfs dataset, whose snapshot is reached in its .zfs directory.
//
// Any other filesystem is refused: a copy asked of a snapshot is not made of
// the live data instead.
//
// Every mount, logical volume, subvolume and zfs snapshot made is written to
// a journal before it is made, and Remove undoes the journal, so a run killed
// before it removed them leaves them to the next one to remove.
package fssnap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/lvm2"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// Set is the snapshots of one copy, as one run of a sync resource.
	Set struct {
		// Dir is the directory of the set, where its journal is kept and
		// its snapshots mounted. It is the set's alone, as the var
		// directory of a resource is.
		Dir string

		// Name names the snapshots, unique to the set among the snapshots
		// of the node: a logical volume, a subvolume and a zfs snapshot
		// are named after it. It holds letters, digits, '_', '.' and '-'.
		Name string

		Log *plog.Logger

		// OneFileSystem says the copy stays on the filesystem of each path,
		// as rsync -x does. Otherwise a filesystem mounted below a path is
		// refused: the snapshot does not hold it, and the copy would take
		// its absence for files to delete.
		OneFileSystem bool
	}

	// entry is something the set made, which Remove undoes.
	entry struct {
		Kind entryKind `json:"kind"`

		// Path is the mount point of a mount, the device of a logical
		// volume, the path of a subvolume relative to the root of its
		// filesystem, the name of a zfs snapshot.
		Path string `json:"path"`

		// Device is the device of the btrfs a subvolume is in.
		Device string `json:"device,omitempty"`
	}

	entryKind string

	// holder is the filesystem a path is snapshotted with, and its
	// snapshot once made.
	holder struct {
		mount mountInfo

		// rels is the paths snapshotted with it, relative to its mount
		// point.
		rels []string

		// root is the directory the snapshot shows the mount point as.
		root string
	}
)

const (
	kindMount       entryKind = "mount"
	kindDir         entryKind = "dir"
	kindLV          entryKind = "lv"
	kindBtrfsSubvol entryKind = "btrfs_subvol"
	kindZFS         entryKind = "zfs"

	journalFile = "journal.json"
)

// Create snapshots the filesystems holding paths, after removing what a run
// killed before its end left, and returns the paths as the snapshots show
// them, in the order given. A path ending with a '/' keeps it, as rsync
// reads it.
//
// On error, what Create made is left to Remove, which the caller defers as
// soon as it calls Create.
func (t *Set) Create(ctx context.Context, paths []string) ([]string, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	if err := t.Remove(ctx); err != nil {
		return nil, fmt.Errorf("remove the snapshots left by an interrupted run: %w", err)
	}
	mounts, err := readMounts()
	if err != nil {
		return nil, err
	}
	holders := make(map[string]*holder)
	order := make([]*holder, 0)
	of := make([]*holder, len(paths))
	rels := make([]string, len(paths))
	for i, p := range paths {
		real, err := resolve(p)
		if err != nil {
			return nil, err
		}
		m, ok := holdingMount(mounts, real)
		if !ok {
			return nil, fmt.Errorf("%s: no filesystem holds it", p)
		}
		if !t.OneFileSystem {
			if below, ok := mountBelow(mounts, real, m, t.Dir); ok {
				return nil, fmt.Errorf("%s holds the mount point %s, which a snapshot of %s does not hold: set the copy to stay on one filesystem", p, below.Target, m.Target)
			}
		}
		h, ok := holders[m.Target]
		if !ok {
			h = &holder{mount: m}
			holders[m.Target] = h
			order = append(order, h)
		}
		of[i] = h
		rels[i] = relUnder(real, m.Target)
		h.rels = append(h.rels, rels[i])
	}
	for i, h := range order {
		if err := t.snapshot(ctx, h, i); err != nil {
			return nil, fmt.Errorf("snapshot %s (%s on %s): %w", h.mount.Target, h.mount.FSType, h.mount.Source, err)
		}
	}
	l := make([]string, len(paths))
	for i, p := range paths {
		l[i] = filepath.Join(of[i].root, rels[i])
		if strings.HasSuffix(p, "/") {
			l[i] += "/"
		}
	}
	return l, nil
}

// resolve returns the path p names, its links resolved as a copy reads it: a
// link named as the source is the link itself, which the copy copies as a
// link, unless the path ends with a '/', which the copy follows to the
// directory it points at. Only the directories it is in are resolved then,
// so the snapshot shows the link where the source shows it, a link pointing
// nowhere included.
func resolve(p string) (string, error) {
	if strings.HasSuffix(p, "/") {
		return filepath.EvalSymlinks(p)
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.Base(p)), nil
}

func (t *Set) check() error {
	if t.Dir == "" || !filepath.IsAbs(t.Dir) {
		return fmt.Errorf("the snapshot set directory must be an absolute path: %q", t.Dir)
	}
	if t.Name == "" || t.Name == "." || t.Name == ".." || strings.Trim(t.Name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.-") != "" {
		return fmt.Errorf("the snapshot set name must hold letters, digits, '_', '.' and '-': %q", t.Name)
	}
	return nil
}

func (t *Set) snapshot(ctx context.Context, h *holder, i int) error {
	switch h.mount.FSType {
	case "btrfs":
		return t.snapshotBtrfs(ctx, h, i)
	case "zfs":
		return t.snapshotZFS(ctx, h)
	case "vxfs":
		return fmt.Errorf("vxfs snapshots are not supported")
	}
	lv, err := lvm2.LVOfDevice(ctx, h.mount.Source, lvm2.WithLogger(t.Log))
	if err != nil {
		return err
	}
	if lv == nil {
		return fmt.Errorf("not a logical volume, a btrfs or a zfs dataset: no snapshot can be taken")
	}
	return t.snapshotLV(ctx, h, i, lv)
}

// mountDir is where the set mounts what it mounts, the i-th of its mounts.
func (t *Set) mountDir(i int) string {
	return filepath.Join(t.Dir, "mnt", fmt.Sprint(i))
}

// makeMountDir creates the directory a mount is made on, recorded so that
// Remove removes it once the mount is undone.
func (t *Set) makeMountDir(p string) error {
	if err := t.record(entry{Kind: kindDir, Path: p}); err != nil {
		return err
	}
	return os.MkdirAll(p, 0700)
}

// Pending says a run left snapshots for the next one to remove, which a
// status can tell.
func Pending(dir string) bool {
	l, err := readJournal(dir)
	return err == nil && len(l) > 0
}

// Remove undoes what the set made, its journal says, the last made first.
// What it could not undo stays in the journal, for the next Remove.
func (t *Set) Remove(ctx context.Context) error {
	l, err := readJournal(t.Dir)
	if err != nil {
		return err
	}
	if len(l) == 0 {
		return t.sweep(ctx)
	}
	var errs error
	left := make([]entry, 0)
	for i := len(l) - 1; i >= 0; i-- {
		e := l[i]
		if err := t.undo(ctx, e); err != nil {
			errs = errors.Join(errs, err)
			left = append([]entry{e}, left...)
		}
	}
	if err := writeJournal(t.Dir, left); err != nil {
		errs = errors.Join(errs, err)
	}
	if len(left) == 0 {
		if err := t.sweep(ctx); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

// RemoveDetached is Remove with a context of its own, for a deferred call
// after a context that may be done.
func (t *Set) RemoveDetached() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return t.Remove(ctx)
}

func (t *Set) undo(ctx context.Context, e entry) error {
	switch e.Kind {
	case kindMount:
		return t.undoMount(ctx, e.Path)
	case kindDir:
		if err := os.Remove(e.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	case kindLV:
		return t.undoLV(ctx, e.Path)
	case kindBtrfsSubvol:
		return t.undoBtrfsSubvol(ctx, e)
	case kindZFS:
		return t.undoZFS(ctx, e.Path)
	default:
		return fmt.Errorf("journal of %s: unknown entry kind %q", t.Dir, e.Kind)
	}
}

// record appends what the set is about to make to its journal, before it
// makes it: a run killed in between leaves an entry for something that may
// not exist, which undoing tolerates, rather than something no entry names.
func (t *Set) record(e entry) error {
	l, err := readJournal(t.Dir)
	if err != nil {
		return err
	}
	return writeJournal(t.Dir, append(l, e))
}

func readJournal(dir string) ([]entry, error) {
	b, err := os.ReadFile(filepath.Join(dir, journalFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var l []entry
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("journal of %s: %w", dir, err)
	}
	return l, nil
}

func writeJournal(dir string, l []entry) error {
	p := filepath.Join(dir, journalFile)
	if len(l) == 0 {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp := p + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	// The entry must be on disk before what it names is made.
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
