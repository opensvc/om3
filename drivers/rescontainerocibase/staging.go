package rescontainerocibase

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/util/confined"
)

// stagingRoot holds the staging mounts, and is replaced by the tests.
//
// It is under /run, which only root writes and which is emptied at boot: a
// staging mount left behind by an agent that died does not survive a reboot.
// It is a directory of its own rather than one under /run/opensvc, which is
// private to root: a rootless engine traverses the staging root to reach its
// mounts.
var stagingRoot = "/run/opensvc-mnt"

// stagingDir holds the staging mounts of the container.
func (t *BT) stagingDir() string {
	return filepath.Join(stagingRoot, t.Path.Namespace, t.Path.Kind.String(), t.Path.Name, t.RID())
}

// stagingPath is the staging mount of the volume_mounts entry at index i.
func (t *BT) stagingPath(i int) string {
	return filepath.Join(t.stagingDir(), strconv.Itoa(i))
}

// stageVolumeMounts makes the staging mount of every volume_mounts entry
// naming a volume, in place of the ones the container had.
//
// The containers mounting a volume write in it, so a path in the volume
// handed to the engine is a path whoever writes there can swap a link into,
// between the check of the agent and the mount of the engine: a link to /
// mounted the whole node in the next container started. So the engine is
// handed no path in the volume. The agent opens the source through a tree
// confined to the head of the volume, which follows no link out of it, and
// bind mounts what it opened, through /proc/self/fd, onto a staging path in
// a directory only root writes. The kernel follows that link to the file the
// agent opened, not to whatever the path leads to by then, so no swap after
// the open changes what is mounted. The engine mounts the staging path, which
// holds no link to follow.
//
// The /proc/self/fd bind works on the kernels of RHEL 7 and later, where the
// mount api of open_tree and move_mount needs 5.2.
//
// The staging mounts are made again at every start, and kept while the
// container exists: an engine restarting a container mounts its sources
// again.
func (t *BT) stageVolumeMounts() error {
	if err := t.unstageVolumeMounts(); err != nil {
		return err
	}
	for i, s := range t.VolumeMounts {
		source, _, _, err := parseVolumeMount(s)
		if err != nil {
			return err
		}
		if strings.HasPrefix(source, "/") {
			// A path of the node, which only root may write, and which
			// is mounted as named.
			continue
		}
		hostPath, vol, err := volumeHostPath(t, source)
		if err != nil {
			return err
		}
		if err := stageMount(vol.Head(), hostPath, t.stagingPath(i)); err != nil {
			return fmt.Errorf("volume_mounts entry %s: %w", s, err)
		}
	}
	return nil
}

// unstageVolumeMounts removes the staging mounts of the container, and does
// nothing for a container that has none, like one an agent without staging
// mounts started.
func (t *BT) unstageVolumeMounts() error {
	dir := t.stagingDir()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	var errs error
	for _, entry := range entries {
		p := filepath.Join(dir, entry.Name())
		if err := unmountAll(p); err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = errors.Join(errs, err)
		}
	}
	if errs != nil {
		return errs
	}
	if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// The directories of the object are removed when they hold nothing
	// else, up to the staging root, which stays.
	for p := filepath.Dir(dir); p != stagingRoot && strings.HasPrefix(p, stagingRoot+string(filepath.Separator)); p = filepath.Dir(p) {
		if os.Remove(p) != nil {
			break
		}
	}
	return nil
}

// unmountAll removes every mount stacked on p, detaching the busy ones.
func unmountAll(p string) error {
	for i := 0; i < 16; i++ {
		err := unix.Unmount(p, unix.MNT_DETACH)
		switch {
		case err == nil:
			continue
		case errors.Is(err, unix.EINVAL), errors.Is(err, unix.ENOENT):
			// Not a mount point, or no longer one.
			return nil
		default:
			return fmt.Errorf("unmount %s: %w", p, err)
		}
	}
	return fmt.Errorf("unmount %s: still mounted", p)
}

// stageMount bind mounts the source, a path in the head of a volume, onto the
// staging path target, without following a link out of the head.
//
// A source missing is made in the head as a directory, as the engines make a
// missing bind source.
func stageMount(head, source, target string) error {
	if head == "" {
		return fmt.Errorf("the volume has no head")
	}
	tree, err := confined.Open(head)
	if err != nil {
		return err
	}
	defer func() { _ = tree.Close() }()
	if _, err := tree.Stat(source); errors.Is(err, os.ErrNotExist) {
		if err := tree.MkdirAll(source, os.ModePerm); err != nil {
			return fmt.Errorf("create the mount source %s: %w", source, err)
		}
	} else if err != nil {
		return fmt.Errorf("mount source %s: %w", source, err)
	}
	f, err := tree.Open(source)
	if err != nil {
		return fmt.Errorf("mount source %s: %w", source, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	// The directories on the way to the staging path are root's, and
	// traversable, so a rootless engine reaches the staging path and
	// changes nothing on the way.
	if err := os.MkdirAll(filepath.Dir(target), 0711); err != nil {
		return err
	}
	if err := traversable(filepath.Dir(target)); err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.Mkdir(target, 0711); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
	} else if mp, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, 0600); err != nil {
		return err
	} else {
		_ = mp.Close()
	}
	fdPath := fmt.Sprintf("/proc/self/fd/%d", f.Fd())
	if err := unix.Mount(fdPath, target, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("bind %s on %s: %w", source, target, err)
	}
	// The staging mount is the container's alone: a mount made in the
	// volume afterwards is not one it was given.
	if err := unix.Mount("", target, "", unix.MS_PRIVATE|unix.MS_REC, ""); err != nil {
		_ = unmountAll(target)
		return fmt.Errorf("make %s private: %w", target, err)
	}
	return nil
}

// traversable makes the directories from the staging root to dir searchable
// by anyone, and writable by root alone, whatever mode they were made with.
func traversable(dir string) error {
	rel, err := filepath.Rel(stagingRoot, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s is not under the staging root %s", dir, stagingRoot)
	}
	p := stagingRoot
	if err := os.Chmod(p, 0711); err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	for _, name := range strings.Split(rel, string(filepath.Separator)) {
		p = filepath.Join(p, name)
		if err := os.Chmod(p, 0711); err != nil {
			return err
		}
	}
	return nil
}
