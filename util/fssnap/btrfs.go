package fssnap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/util/btrfs"
)

// btrfsSnapDir is the directory of the root of a btrfs the snapshots are
// made in, outside of the subvolumes mounted.
const btrfsSnapDir = ".osvc-fssnap"

// snapshotBtrfs snapshots, read-only, the subvolume of the btrfs mounted,
// through a mount of the root of the filesystem, the subvolumes are reached
// from wherever they are.
func (t *Set) snapshotBtrfs(ctx context.Context, h *holder, i int) error {
	dev := h.mount.Source
	p, err := t.mount(ctx, i, "btrfs", "subvolid=5", dev)
	if err != nil {
		return err
	}
	subvol := strings.Trim(h.mount.Root, "/")
	if !t.OneFileSystem {
		if err := checkNoNestedSubvol(ctx, p, subvol, h.rels); err != nil {
			return err
		}
	}
	rel := filepath.Join(btrfsSnapDir, t.Name)
	if err := t.record(entry{Kind: kindBtrfsSubvol, Path: rel, Device: dev}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(p, btrfsSnapDir), 0700); err != nil {
		return err
	}
	if _, err := run(ctx, t.Log, "btrfs", "subvolume", "snapshot", "-r", filepath.Join(p, subvol), filepath.Join(p, rel)); err != nil {
		return err
	}
	h.root = filepath.Join(p, rel)
	return nil
}

// checkNoNestedSubvol refuses a subvolume nested in subvol below one of the
// paths, rels relative to subvol: a snapshot holds it as an empty directory,
// and the copy would take its absence for files to delete.
func checkNoNestedSubvol(ctx context.Context, root, subvol string, rels []string) error {
	b, err := probe(ctx, "btrfs", btrfs.ListArgs(false, root)...)
	if err != nil {
		return err
	}
	l, err := btrfs.ParseList(b)
	if err != nil {
		return err
	}
	for _, s := range l {
		rel, ok := strings.CutPrefix(s.Path, subvol+"/")
		if subvol == "" {
			rel, ok = s.Path, true
		}
		if !ok || isUnder(rel, btrfsSnapDir) {
			continue
		}
		for _, r := range rels {
			if r == "." || isUnder(rel, r) {
				return fmt.Errorf("the subvolume %s is nested below %s, and a snapshot does not hold it: set the copy to stay on one filesystem", s.Path, r)
			}
		}
	}
	return nil
}

// undoBtrfsSubvol deletes the snapshot subvolume e names, through a mount of
// the root of its filesystem: the one the set made, when still mounted, or
// one made for the deletion, the run that made the other having been killed.
func (t *Set) undoBtrfsSubvol(ctx context.Context, e entry) error {
	root, done, err := t.btrfsRoot(ctx, e.Device)
	if err != nil {
		return err
	}
	var errs error
	p := filepath.Join(root, e.Path)
	if _, err := os.Stat(p); err == nil {
		if _, err := run(ctx, t.Log, "btrfs", "subvolume", "delete", p); err != nil {
			errs = errors.Join(errs, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		errs = errors.Join(errs, err)
	}
	if errs == nil {
		// The directory the snapshots are made in, once the last is
		// deleted. Another set may still have one in it.
		_ = os.Remove(filepath.Dir(p))
	}
	return errors.Join(errs, done())
}

// btrfsRoot returns a mount point of the root of the btrfs on dev, made by
// the set, and what to call once done with it.
func (t *Set) btrfsRoot(ctx context.Context, dev string) (string, func() error, error) {
	mounts, err := readMounts()
	if err != nil {
		return "", nil, err
	}
	for _, m := range mounts {
		if m.Source == dev && m.FSType == "btrfs" && m.Root == "/" && isUnder(m.Target, t.Dir) {
			return m.Target, func() error { return nil }, nil
		}
	}
	// Not journaled: the sweep of Remove unmounts it if the run is killed
	// before it does.
	p := filepath.Join(t.Dir, "cleanup")
	if err := os.MkdirAll(p, 0700); err != nil {
		return "", nil, err
	}
	if _, err := run(ctx, t.Log, "mount", "-t", "btrfs", "-o", "subvolid=5", dev, p); err != nil {
		_ = os.Remove(p)
		return "", nil, err
	}
	return p, func() error {
		if err := t.undoMount(ctx, p); err != nil {
			return err
		}
		return os.Remove(p)
	}, nil
}
