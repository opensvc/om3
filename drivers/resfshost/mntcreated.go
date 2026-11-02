package resfshost

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opensvc/om3/v3/util/confined"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// MountPointRecord is the record of the mount point a start of a
	// resource created, and of the directories created with it, the
	// missing parents a MkdirAll made: what the unprovision may remove,
	// each when empty and still the one created.
	//
	// The file holds the mount point and its identity, its device and its
	// inode, then a line per parent created, its path and its identity, from
	// the deepest. A path replaced since is told apart by its identity.
	MountPointRecord struct {
		File string
		Log  *plog.Logger
	}

	// mountPointPaths is how the paths of a mount point are reached: on the
	// host, or through the root of a container, which a link of the
	// container does not lead out of.
	mountPointPaths interface {
		// exists says something is at the path, a link not followed.
		exists(p string) bool
		// identity returns the device and the inode of the path, a link
		// not followed.
		identity(p string) (string, error)
		// removeEmptyDir removes the directory at the path when it is
		// empty, and fails otherwise.
		removeEmptyDir(p string) error
		// top is the path no parent is looked for above.
		top() string
	}

	hostPaths struct{}

	zonePaths struct{ z Zone }
)

// MissingParents returns the parents of mnt a MkdirAll would create, from the
// deepest, to call before it creates them.
func MissingParents(mnt string) []string {
	return missingParents(hostPaths{}, mnt)
}

// MissingParentsInZone returns the parents of the mount point of z in the
// container a MkdirAll would create, as host paths, from the deepest.
func MissingParentsInZone(z Zone) []string {
	return missingParents(zonePaths{z}, z.Path())
}

func missingParents(paths mountPointPaths, mnt string) []string {
	l := make([]string, 0)
	for p := filepath.Dir(mnt); p != paths.top() && p != filepath.Dir(p); p = filepath.Dir(p) {
		if paths.exists(p) {
			break
		}
		l = append(l, p)
	}
	return l
}

// Mark records mnt created by a start, with the parents created with it.
func (r MountPointRecord) Mark(mnt string, parents []string) {
	r.mark(hostPaths{}, mnt, parents)
}

// MarkInZone records the mount point of z created by a start, with the
// parents created with it.
func (r MountPointRecord) MarkInZone(z Zone, parents []string) {
	r.mark(zonePaths{z}, z.Path(), parents)
}

func (r MountPointRecord) mark(paths mountPointPaths, mnt string, parents []string) {
	identity, err := paths.identity(mnt)
	if err != nil {
		r.Log.Warnf("record the mount point %s created: %s", mnt, err)
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n", mnt, identity)
	for _, p := range parents {
		id, err := paths.identity(p)
		if err != nil {
			// Not made by the MkdirAll: nothing above it was either.
			break
		}
		fmt.Fprintf(&b, "%s\t%s\n", p, id)
	}
	if err := os.MkdirAll(filepath.Dir(r.File), 0o700); err != nil {
		r.Log.Warnf("record the mount point %s created: %s", mnt, err)
		return
	}
	if err := os.WriteFile(r.File, []byte(b.String()), 0o600); err != nil {
		r.Log.Warnf("record the mount point %s created: %s", mnt, err)
	}
}

// Forget removes the record.
func (r MountPointRecord) Forget() {
	if err := os.Remove(r.File); err != nil && !errors.Is(err, os.ErrNotExist) {
		r.Log.Warnf("forget the mount point created: %s", err)
	}
}

// read returns the mount point recorded, its identity, and the parents
// created with it, from the deepest. A record of an agent knowing no parent
// has none.
func (r MountPointRecord) read() (mnt, identity string, parents [][2]string, ok bool) {
	b, err := os.ReadFile(r.File)
	if err != nil {
		return "", "", nil, false
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 {
		return "", "", nil, false
	}
	for _, line := range lines[2:] {
		if p, id, ok := strings.Cut(line, "\t"); ok {
			parents = append(parents, [2]string{p, id})
		}
	}
	return lines[0], lines[1], parents, true
}

// Created says if what is at mnt is the mount point a start created: a
// change of mnt, or a path replaced since, as by an operator once the
// resource stopped, leaves it to the operator, and the record is forgotten.
func (r MountPointRecord) Created(mnt string) bool {
	return r.created(hostPaths{}, mnt)
}

func (r MountPointRecord) created(paths mountPointPaths, mnt string) bool {
	recorded, identity, _, ok := r.read()
	if !ok || recorded != mnt {
		return false
	}
	current, err := paths.identity(mnt)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil || current != identity {
		r.Log.Infof("leave the mount point %s: it is not the one the resource created", mnt)
		r.Forget()
		return false
	}
	return true
}

// Remove removes the mount point mnt the record says a start created, when
// it is empty, then each parent created with it, from the deepest, when it is
// empty and still the one created. A directory left stops the removal of the
// parents above it. A protected path is never removed.
func (r MountPointRecord) Remove(mnt string) {
	r.remove(hostPaths{}, mnt)
}

// RemoveInZone removes the mount point of z, and the parents created with it,
// through the root of the container. The root itself is never removed.
func (r MountPointRecord) RemoveInZone(z Zone) {
	if z.IsRoot() {
		return
	}
	r.remove(zonePaths{z}, z.Path())
}

func (r MountPointRecord) remove(paths mountPointPaths, mnt string) {
	recorded, _, parents, ok := r.read()
	if !ok || recorded != mnt {
		return
	}
	if paths.exists(mnt) {
		if !r.created(paths, mnt) {
			return
		}
		if file.IsProtected(mnt) {
			return
		}
		if err := paths.removeEmptyDir(mnt); err != nil {
			r.Log.Infof("leave the mount point %s: %s", mnt, err)
			return
		}
		r.Log.Infof("removed the mount point %s", mnt)
	}
	for _, parent := range parents {
		p, id := parent[0], parent[1]
		if !paths.exists(p) {
			continue
		}
		if file.IsProtected(p) {
			break
		}
		if current, err := paths.identity(p); err != nil || current != id {
			r.Log.Infof("leave the directory %s: it is not the one the resource created", p)
			break
		}
		if err := paths.removeEmptyDir(p); err != nil {
			r.Log.Infof("leave the directory %s: %s", p, err)
			break
		}
		r.Log.Infof("removed the directory %s, created with the mount point", p)
	}
	r.Forget()
}

func (hostPaths) top() string { return "/" }

func (hostPaths) exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func (hostPaths) identity(p string) (string, error) {
	return mountPointIdentity(p)
}

// removeEmptyDir removes a directory, or the empty file of a file bind mount,
// as rmdir and unlink do: what holds something is left.
func (hostPaths) removeEmptyDir(p string) error {
	fi, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if !fi.IsDir() && !(fi.Mode().IsRegular() && fi.Size() == 0) {
		return fmt.Errorf("%s is no directory", p)
	}
	return os.Remove(p)
}

func (t zonePaths) top() string { return t.z.Root }

// rel returns the path in the container of a host path under its root.
func (t zonePaths) rel(p string) (string, bool) {
	rel, err := filepath.Rel(t.z.Root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return filepath.Clean("/" + rel), true
}

// lstat looks at the path in the container, through its parent opened not
// following a link.
func (t zonePaths) lstat(p string) (os.FileInfo, *os.Root, string, error) {
	rel, ok := t.rel(p)
	if !ok || rel == "/" {
		return nil, nil, "", fmt.Errorf("%s: not in the container", p)
	}
	parent, err := confined.OpenDirNoLink(t.z.Root, filepath.Dir(rel), false, 0)
	if err != nil {
		return nil, nil, "", err
	}
	name := filepath.Base(rel)
	fi, err := parent.Lstat(name)
	if err != nil {
		_ = parent.Close()
		return nil, nil, "", err
	}
	return fi, parent, name, nil
}

func (t zonePaths) exists(p string) bool {
	_, parent, _, err := t.lstat(p)
	if err != nil {
		return false
	}
	_ = parent.Close()
	return true
}

func (t zonePaths) identity(p string) (string, error) {
	fi, parent, _, err := t.lstat(p)
	if err != nil {
		return "", err
	}
	_ = parent.Close()
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("%s: no device and inode", p)
	}
	return fmt.Sprintf("%d %d", st.Dev, st.Ino), nil
}

func (t zonePaths) removeEmptyDir(p string) error {
	fi, parent, name, err := t.lstat(p)
	if err != nil {
		return err
	}
	defer func() { _ = parent.Close() }()
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a link", p)
	}
	if !fi.IsDir() && !(fi.Mode().IsRegular() && fi.Size() == 0) {
		return fmt.Errorf("%s is no directory", p)
	}
	return parent.Remove(name)
}
