package confined

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// OpenDirNoLink opens the directory rel under the directory dir, one
// component after the other, each through the one before, refusing a link,
// even one leading back into dir, and a directory swapped between its check
// and its opening. A missing component is made with perm when create is
// set, and is an error otherwise.
//
// It is for the paths a root process acts on by name afterwards, as a mount
// point, where a link staying in the tree would still move the path away
// from the one configured.
func OpenDirNoLink(dir, rel string, create bool, perm fs.FileMode) (*os.Root, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	sub, err := OpenRootNoLink(root, rel, create, perm)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, rel), err)
	}
	return sub, nil
}

// OpenRootNoLink opens the directory rel under root as OpenDirNoLink does.
// The root is left open, and the root returned is a new one, to close.
func OpenRootNoLink(root *os.Root, rel string, create bool, perm fs.FileMode) (*os.Root, error) {
	r, err := root.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	for _, name := range components(rel) {
		next, err := openComponent(r, name, create, perm)
		_ = r.Close()
		if err != nil {
			return nil, err
		}
		r = next
	}
	return r, nil
}

// CheckNoLink walks rel under dir, up to its first missing component,
// refusing a link: the path, as it is now, is the one it names.
func CheckNoLink(dir, rel string) error {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	for _, name := range components(rel) {
		fi, err := r.Lstat(name)
		switch {
		case errors.Is(err, os.ErrNotExist):
			return nil
		case err != nil:
			return fmt.Errorf("%s: %w", filepath.Join(dir, rel), err)
		case fi.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("%s: %s is a link", filepath.Join(dir, rel), name)
		case !fi.IsDir():
			// The last component may be a file, as the mount point
			// of a file bind mount.
			return nil
		}
		next, err := openComponent(r, name, false, 0)
		_ = r.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Join(dir, rel), err)
		}
		r = next
	}
	return nil
}

// components returns the names of the path rel, relative or not, without
// the empty ones and "..", which Clean resolves against the start.
func components(rel string) []string {
	l := make([]string, 0)
	for _, name := range strings.Split(filepath.Clean("/"+rel), string(filepath.Separator)) {
		if name != "" {
			l = append(l, name)
		}
	}
	return l
}

// openComponent opens the directory name of root, made with perm when
// absent and create is set, refusing a link and a directory swapped between
// its check and its opening.
func openComponent(root *os.Root, name string, create bool, perm fs.FileMode) (*os.Root, error) {
	fi, err := root.Lstat(name)
	if create && errors.Is(err, os.ErrNotExist) {
		if err := root.Mkdir(name, perm); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		fi, err = root.Lstat(name)
	}
	switch {
	case err != nil:
		return nil, err
	case fi.Mode()&os.ModeSymlink != 0:
		return nil, fmt.Errorf("%s is a link", name)
	case !fi.IsDir():
		return nil, fmt.Errorf("%s is not a directory", name)
	}
	sub, err := root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, err := sub.Stat(".")
	if err != nil {
		_ = sub.Close()
		return nil, err
	}
	if !os.SameFile(fi, opened) {
		_ = sub.Close()
		return nil, fmt.Errorf("%s changed while opened", name)
	}
	return sub, nil
}
