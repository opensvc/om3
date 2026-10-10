package fssnap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opensvc/om3/v3/util/filesystems"
	"github.com/opensvc/om3/v3/util/mountinfo"
	"github.com/opensvc/om3/v3/util/plog"
)

// mountInfo is a mount of the node.
type mountInfo = mountinfo.Entry

var (
	// readMounts returns the mounts of the node, replaced by the tests.
	readMounts = mountinfo.Read

	// run runs a command and returns its stdout, replaced by the tests.
	// Its error says what the command wrote on stderr.
	run = func(ctx context.Context, log *plog.Logger, name string, args ...string) ([]byte, error) {
		var stdout, stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		log.Infof("%s", cmd)
		if err := cmd.Run(); err != nil {
			return stdout.Bytes(), fmt.Errorf("%s: %w: %s", cmd, err, strings.TrimSpace(stderr.String()))
		}
		return stdout.Bytes(), nil
	}

	// probe runs a command whose failure is an answer, as a lookup of what
	// does not exist, and does not log it, replaced by the tests.
	probe = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		var stdout, stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return stdout.Bytes(), fmt.Errorf("%s: %w: %s", cmd, err, strings.TrimSpace(stderr.String()))
		}
		return stdout.Bytes(), nil
	}
)

// isUnder says p is dir or a path below it.
func isUnder(p, dir string) bool {
	if dir == "/" {
		return strings.HasPrefix(p, "/")
	}
	return p == dir || strings.HasPrefix(p, dir+"/")
}

// holdingMount is the mount p is on: the deepest mount point p is under,
// the last mounted of the mounts on one mount point, which hides the others.
func holdingMount(mounts []mountInfo, p string) (mountInfo, bool) {
	var (
		found mountInfo
		ok    bool
	)
	for _, m := range mounts {
		if !isUnder(p, m.Target) {
			continue
		}
		if !ok || len(m.Target) >= len(found.Target) {
			found, ok = m, true
		}
	}
	return found, ok
}

// mountBelow is a mount other than holding with its mount point below p,
// but for the mounts below exclude, the mounts of a set, and the snapshots of
// a zfs, which it mounts when they are read.
func mountBelow(mounts []mountInfo, p string, holding mountInfo, exclude string) (mountInfo, bool) {
	for _, m := range mounts {
		if m.Target == holding.Target || isUnder(m.Target, exclude) {
			continue
		}
		if m.FSType == "zfs" && strings.Contains(m.Target, "/.zfs/snapshot/") {
			continue
		}
		if isUnder(m.Target, p) && m.Target != p {
			return m, true
		}
	}
	return mountInfo{}, false
}

// mount mounts dev, a filesystem of type fsType, on the i-th mount
// directory of the set, with opts, and returns the mount point.
func (t *Set) mount(ctx context.Context, i int, fsType, opts, dev string) (string, error) {
	fs := filesystems.FromType(fsType)
	if fs.Type() != fsType {
		return "", fmt.Errorf("no driver mounts a %s filesystem", fsType)
	}
	fs.SetLog(t.Log)
	p := t.mountDir(i)
	if err := t.makeMountDir(p); err != nil {
		return "", err
	}
	if err := t.record(entry{Kind: kindMount, Path: p}); err != nil {
		return "", err
	}
	if err := fs.Mount(ctx, dev, p, opts); err != nil {
		return "", err
	}
	return p, nil
}

// undoMount unmounts p, killing the processes holding it when they keep it
// busy: a copy killed with the run that started it may still read it. Only a
// mount of the set is undone.
func (t *Set) undoMount(ctx context.Context, p string) error {
	if !isUnder(p, t.Dir) || p == t.Dir {
		return fmt.Errorf("refuse to unmount %s, which is not a mount of the snapshot set in %s", p, t.Dir)
	}
	m, ok, err := mountOf(p)
	if err != nil {
		return err
	} else if !ok {
		return nil
	}
	fs := filesystems.FromType(m.FSType)
	fs.SetLog(t.Log)
	err = fs.Umount(ctx, p)
	if err == nil {
		return nil
	} else if !errors.Is(err, syscall.EBUSY) {
		return err
	}
	t.Log.Warnf("%s", err)
	if killHolders(t.Log, p) == 0 {
		return err
	}
	return fs.Umount(ctx, p)
}

// mountOf returns the mount on p.
func mountOf(p string) (mountInfo, bool, error) {
	mounts, err := readMounts()
	if err != nil {
		return mountInfo{}, false, err
	}
	for i := len(mounts) - 1; i >= 0; i-- {
		if mounts[i].Target == p {
			return mounts[i], true, nil
		}
	}
	return mountInfo{}, false, nil
}

// sweep unmounts what is still mounted below the set directory, which no
// journal entry names: a mount made to undo something, by a run killed
// before it unmounted it.
func (t *Set) sweep(ctx context.Context) error {
	mounts, err := readMounts()
	if err != nil {
		return err
	}
	var errs error
	// The deepest first.
	for i := len(mounts) - 1; i >= 0; i-- {
		m := mounts[i]
		if !isUnder(m.Target, t.Dir) || m.Target == t.Dir {
			continue
		}
		if err := t.undoMount(ctx, m.Target); err != nil {
			errs = errors.Join(errs, err)
		} else if err := os.Remove(m.Target); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

// relUnder is p relative to dir, "." for dir itself.
func relUnder(p, dir string) string {
	rel, err := filepath.Rel(dir, p)
	if err != nil {
		return p
	}
	return rel
}
