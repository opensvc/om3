package resfshost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/confined"
)

// zoneContainer is a container resource whose root directory a filesystem
// can be mounted under, as the container.lxc one.
type zoneContainer interface {
	ContainerName() string
	ContainerHead(ctx context.Context) (string, error)
}

// Zone is the mount point of a filesystem in the root directory of a
// container, as the zone keyword asks: the root of the container on the
// host, and the path in it.
//
// The container writes under its root, a process of it can make a component
// of the path a link to a host path, and the agent acts on the mount point
// as root: so every operation walks the path from the root one component
// after the other, refusing a link, and the mount and the umount are bound
// to the directories opened, rather than to the path.
type Zone struct {
	Root string
	Rel  string
}

// ZoneOf returns the mount point mnt of a filesystem of the object o in the
// root directory of the container zone names: a container resource of the
// object holding a root directory, named by its name or its rid, as v2 did.
//
// A zone naming no such container is an error, rather than the filesystem
// mounted on the host path, outside the container it was meant for.
func ZoneOf(ctx context.Context, o resource.ObjectDriver, zone, mnt string) (Zone, error) {
	if o == nil {
		return Zone{}, fmt.Errorf("zone %s: no object to find the container in", zone)
	}
	for _, r := range o.ResourcesByDrivergroups([]driver.Group{driver.GroupContainer}) {
		c, ok := r.(zoneContainer)
		if !ok {
			continue
		}
		if c.ContainerName() != zone && r.RID() != zone {
			continue
		}
		root, err := c.ContainerHead(ctx)
		if err != nil {
			return Zone{}, fmt.Errorf("zone %s: %w", zone, err)
		}
		if !filepath.IsAbs(root) {
			return Zone{}, fmt.Errorf("zone %s: the root directory %s is not an absolute path", zone, root)
		}
		return Zone{Root: filepath.Clean(root), Rel: filepath.Clean("/" + mnt)}, nil
	}
	return Zone{}, fmt.Errorf("zone %s: no container resource of the object holding a root directory is named so", zone)
}

// Path returns the mount point on the host.
func (z Zone) Path() string {
	return filepath.Join(z.Root, z.Rel)
}

// IsRoot says the mount point is the root of the container itself.
func (z Zone) IsRoot() bool {
	return z.Rel == "/"
}

// Check refuses a link on the path of the mount point in the container, as
// it is now.
func (z Zone) Check() error {
	return confined.CheckNoLink(z.Root, z.Rel)
}

// MkdirAll makes the mount point directory, and the missing ones on its
// path, with perm, in the container.
func (z Zone) MkdirAll(perm os.FileMode) error {
	r, err := confined.OpenDirNoLink(z.Root, z.Rel, true, perm)
	if err != nil {
		return err
	}
	return r.Close()
}

// CreateFile makes the empty file of the mount point of a file bind mount,
// and the missing directories on its path, in the container.
func (z Zone) CreateFile(perm os.FileMode) error {
	if z.IsRoot() {
		return fmt.Errorf("%s: the root of the container is not a file", z.Path())
	}
	parent, err := confined.OpenDirNoLink(z.Root, filepath.Dir(z.Rel), true, perm)
	if err != nil {
		return err
	}
	defer parent.Close()
	name := filepath.Base(z.Rel)
	fi, err := parent.Lstat(name)
	switch {
	case err == nil && fi.Mode().IsRegular():
		return nil
	case err == nil:
		return fmt.Errorf("%s already exists but is not a regular file", z.Path())
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	f, err := parent.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

// Open opens the mount point, a directory or the file of a file bind mount,
// for a mount through /proc/<pid>/fd/<fd>: the kernel follows it to what was
// opened, whatever the path leads to by then.
func (z Zone) Open() (*os.File, error) {
	if z.IsRoot() {
		r, err := os.OpenRoot(z.Root)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return r.Open(".")
	}
	parent, err := confined.OpenDirNoLink(z.Root, filepath.Dir(z.Rel), false, 0)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	name := filepath.Base(z.Rel)
	fi, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s: %s is a link", z.Path(), name)
	}
	if fi.IsDir() {
		r, err := confined.OpenRootNoLink(parent, name, false, 0)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return r.Open(".")
	}
	// The file of a file bind mount is opened not following a link,
	// and has to be the one checked: a process of the container could
	// swap it for a link, or another file, in between.
	f, err := parent.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", z.Path(), err)
	}
	if opened, err := f.Stat(); err != nil {
		_ = f.Close()
		return nil, err
	} else if !os.SameFile(fi, opened) {
		_ = f.Close()
		return nil, fmt.Errorf("%s changed while opened", z.Path())
	}
	return f, nil
}

// FDPath returns the path the kernel follows to the file f, from another
// process too, as mount(8).
func FDPath(f *os.File) string {
	return fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), f.Fd())
}

// UmountTarget returns the path to unmount the mount point by: its name in
// its parent directory, opened, which a link swapped in at the name does not
// redirect when unmounted with UMOUNT_NOFOLLOW. The parent is to be closed
// once unmounted.
func (z Zone) UmountTarget() (string, *os.File, error) {
	if z.IsRoot() {
		return z.Root, nil, nil
	}
	parent, err := confined.OpenDirNoLink(z.Root, filepath.Dir(z.Rel), false, 0)
	if err != nil {
		return "", nil, err
	}
	defer parent.Close()
	f, err := parent.Open(".")
	if err != nil {
		return "", nil, err
	}
	return filepath.Join(FDPath(f), filepath.Base(z.Rel)), f, nil
}

// Remove removes the mount point, when empty, in the container. The root of
// the container itself is never removed.
func (z Zone) Remove() error {
	if z.IsRoot() {
		return fmt.Errorf("%s: the root of the container is not the filesystem's to remove", z.Path())
	}
	parent, err := confined.OpenDirNoLink(z.Root, filepath.Dir(z.Rel), false, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	defer parent.Close()
	name := filepath.Base(z.Rel)
	if fi, err := parent.Lstat(name); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	} else if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s: %s is a link", z.Path(), name)
	}
	return parent.Remove(name)
}

// validateMountPoint refuses a relative mount point: it would resolve
// against the working directory of whichever process runs the action, the
// daemon or a command started anywhere, and the status, comparing it to the
// absolute paths of the mount table, would never see it mounted. With a
// zone, the mount point is the one in the container, absolute too.
func validateMountPoint(s string) error {
	if !filepath.IsAbs(s) {
		return fmt.Errorf("%s is not an absolute path", s)
	}
	return nil
}
