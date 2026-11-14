package confined

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrUntrusted is the error of a path another user could have made or
// changed: a directory or a file owned by another user, a directory others
// can write in.
var ErrUntrusted = errors.New("not trusted")

// OpenDirTrusted opens the directory rel under the directory dir as
// OpenDirNoLink does, and also requires each component under dir to be owned
// by the user of the process and writable by it alone.
//
// It is for the state a root process keeps under a directory every user can
// write in, as /dev/shm or /var/tmp: a user making the path first, or
// writing in one of its directories, could plant a link where the process
// writes, or forge the state it reads. The directory dir is not checked: it
// is the system's, as /dev/shm.
//
// A missing component is made with perm, whatever the umask, when create is
// set. A component owned by the user and writable by others, as one made
// under a umask of 0, is given perm, and tightened is called with its path
// and its former mode. A component owned by another user is ErrUntrusted:
// it is never taken over, an admin removes it.
func OpenDirTrusted(dir, rel string, create bool, perm fs.FileMode, tightened func(path string, mode fs.FileMode)) (*os.Root, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	p := dir
	for _, name := range components(rel) {
		p = filepath.Join(p, name)
		_, statErr := r.Lstat(name)
		made := create && errors.Is(statErr, os.ErrNotExist)
		next, err := openComponent(r, name, create, perm)
		_ = r.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		r = next
		if err := trustDir(r, p, made, perm, tightened); err != nil {
			_ = r.Close()
			return nil, err
		}
	}
	return r, nil
}

// MkdirTrusted makes the directory rel under the directory dir, as
// OpenDirTrusted does.
func MkdirTrusted(dir, rel string, perm fs.FileMode, tightened func(path string, mode fs.FileMode)) error {
	r, err := OpenDirTrusted(dir, rel, true, perm, tightened)
	if err != nil {
		return err
	}
	return r.Close()
}

// trustDir checks the directory r, at path p, is owned by the user of the
// process and writable by it alone, and gives it perm when it was just made
// or was writable by others.
func trustDir(r *os.Root, p string, made bool, perm fs.FileMode, tightened func(path string, mode fs.FileMode)) error {
	fi, err := r.Stat(".")
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	if uid, ok := fileOwner(fi); ok && uid != os.Geteuid() {
		return fmt.Errorf("%s: owned by uid %d: %w", p, uid, ErrUntrusted)
	}
	mode := fi.Mode().Perm()
	switch {
	case made:
	case mode&0o022 != 0:
		if tightened != nil {
			defer tightened(p, mode)
		}
	default:
		return nil
	}
	if err := r.Chmod(".", perm); err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	return nil
}

// TrustedFile says the entry name of the directory r is a regular file owned
// by the user of the process, as the files the process made there. A link,
// which a stat would follow, a directory, a file of another user are not:
// the error tells which. A missing entry is not, with no error.
func TrustedFile(r *os.Root, name string) (bool, error) {
	fi, err := r.Lstat(name)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	case fi.Mode()&fs.ModeSymlink != 0:
		return false, fmt.Errorf("%s is a link: %w", name, ErrUntrusted)
	case fi.IsDir():
		return false, fmt.Errorf("%s is a directory: %w", name, ErrUntrusted)
	case !fi.Mode().IsRegular():
		return false, fmt.Errorf("%s is not a regular file: %w", name, ErrUntrusted)
	}
	if uid, ok := fileOwner(fi); ok && uid != os.Geteuid() {
		return false, fmt.Errorf("%s is owned by uid %d: %w", name, uid, ErrUntrusted)
	}
	return true, nil
}
