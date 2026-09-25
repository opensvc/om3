// Package confined runs file operations on the paths of a directory tree
// without ever leaving it.
//
// The agent writes, as root, in directories others can write in too: the
// head of a volume, which the containers mounting it write in, or the runtime
// directory of a user running rootless containers. A path the agent joins
// under such a directory can be made to lead anywhere by whoever writes
// there: a link planted where the agent installs a file, or a directory on
// the way, makes a write, a chmod or a chown of the agent land on a file of
// the node, like /etc/shadow.
//
// A Tree resolves every path under its directory with os.Root, which refuses
// a path, a link or a ".." leading out of it, and does so for each
// operation, so a link planted between two of them is refused too.
package confined

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type (
	// FS is the file operations the agent runs when it installs files, on
	// absolute paths.
	FS interface {
		Stat(name string) (fs.FileInfo, error)
		Lstat(name string) (fs.FileInfo, error)
		Mkdir(name string, perm fs.FileMode) error
		MkdirAll(name string, perm fs.FileMode) error
		Chmod(name string, mode fs.FileMode) error
		Lchown(name string, uid, gid int) error
		Chtimes(name string, atime, mtime time.Time) error
		ReadFile(name string) ([]byte, error)
		WriteFile(name string, data []byte, perm fs.FileMode) error
		Remove(name string) error
		RemoveAll(name string) error
	}

	// Tree is FS confined to a directory: a path out of it, whether it is
	// named so or leads there through a link, is refused.
	Tree struct {
		dir  string
		root *os.Root
	}

	host struct{}
)

// ErrEscapes is a path leading out of the tree.
var ErrEscapes = errors.New("path escapes the confined tree")

// Host is FS on the whole of the node, for the paths the agent alone writes
// in.
func Host() FS {
	return host{}
}

// Open returns the tree of the directory, to be closed after use.
func Open(dir string) (*Tree, error) {
	dir = filepath.Clean(dir)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &Tree{dir: dir, root: root}, nil
}

// Close releases the directory of the tree.
func (t *Tree) Close() error {
	return t.root.Close()
}

// Dir is the directory of the tree.
func (t *Tree) Dir() string {
	return t.dir
}

// rel returns the path of name relative to the tree, refusing a name out of
// it. Where name leads after its links is for os.Root to judge.
func (t *Tree) rel(name string) (string, error) {
	if !filepath.IsAbs(name) {
		name = filepath.Join(t.dir, name)
	}
	rel, err := filepath.Rel(t.dir, filepath.Clean(name))
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s: %w %s", name, ErrEscapes, t.dir)
	}
	return rel, nil
}

func (t *Tree) Stat(name string) (fs.FileInfo, error) {
	rel, err := t.rel(name)
	if err != nil {
		return nil, err
	}
	return t.root.Stat(rel)
}

func (t *Tree) Lstat(name string) (fs.FileInfo, error) {
	rel, err := t.rel(name)
	if err != nil {
		return nil, err
	}
	return t.root.Lstat(rel)
}

func (t *Tree) Mkdir(name string, perm fs.FileMode) error {
	rel, err := t.rel(name)
	if err != nil {
		return err
	}
	return t.root.Mkdir(rel, perm)
}

func (t *Tree) MkdirAll(name string, perm fs.FileMode) error {
	rel, err := t.rel(name)
	if err != nil {
		return err
	}
	return t.root.MkdirAll(rel, perm)
}

func (t *Tree) Chmod(name string, mode fs.FileMode) error {
	rel, err := t.rel(name)
	if err != nil {
		return err
	}
	return t.root.Chmod(rel, mode)
}

func (t *Tree) Lchown(name string, uid, gid int) error {
	rel, err := t.rel(name)
	if err != nil {
		return err
	}
	return t.root.Lchown(rel, uid, gid)
}

func (t *Tree) Chtimes(name string, atime, mtime time.Time) error {
	rel, err := t.rel(name)
	if err != nil {
		return err
	}
	return t.root.Chtimes(rel, atime, mtime)
}

func (t *Tree) ReadFile(name string) ([]byte, error) {
	rel, err := t.rel(name)
	if err != nil {
		return nil, err
	}
	return t.root.ReadFile(rel)
}

func (t *Tree) WriteFile(name string, data []byte, perm fs.FileMode) error {
	rel, err := t.rel(name)
	if err != nil {
		return err
	}
	return t.root.WriteFile(rel, data, perm)
}

func (t *Tree) Remove(name string) error {
	rel, err := t.rel(name)
	if err != nil {
		return err
	}
	return t.root.Remove(rel)
}

func (t *Tree) RemoveAll(name string) error {
	rel, err := t.rel(name)
	if err != nil {
		return err
	}
	if rel == "." {
		return fmt.Errorf("%s: refuse to remove the confined tree itself", name)
	}
	return t.root.RemoveAll(rel)
}

func (host) Stat(name string) (fs.FileInfo, error)        { return os.Stat(name) }
func (host) Lstat(name string) (fs.FileInfo, error)       { return os.Lstat(name) }
func (host) Mkdir(name string, perm fs.FileMode) error    { return os.Mkdir(name, perm) }
func (host) MkdirAll(name string, perm fs.FileMode) error { return os.MkdirAll(name, perm) }
func (host) Chmod(name string, mode fs.FileMode) error    { return os.Chmod(name, mode) }
func (host) Lchown(name string, uid, gid int) error       { return os.Lchown(name, uid, gid) }
func (host) Chtimes(name string, atime, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}
func (host) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }
func (host) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (host) Remove(name string) error    { return os.Remove(name) }
func (host) RemoveAll(name string) error { return os.RemoveAll(name) }
