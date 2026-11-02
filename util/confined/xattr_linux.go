package confined

import (
	"os"

	"golang.org/x/sys/unix"
)

// xattrMax is the most an extended attribute read here holds.
const xattrMax = 4096

// openNoFollow opens the file at name in the tree for an extended attribute:
// a link is refused rather than followed, which the root of the tree would do
// whatever the flags, and a fifo does not block the open.
func (t *Tree) openNoFollow(name string) (*os.File, error) {
	rel, err := t.rel(name)
	if err != nil {
		return nil, err
	}
	before, err := t.root.Lstat(rel)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 {
		return nil, &os.PathError{Op: "open", Path: name, Err: unix.ELOOP}
	}
	f, err := t.root.OpenFile(rel, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	// Replaced by a link between the two, the file opened is another.
	if after, err := f.Stat(); err != nil || !os.SameFile(before, after) {
		_ = f.Close()
		return nil, &os.PathError{Op: "open", Path: name, Err: unix.ELOOP}
	}
	return f, nil
}

func (t *Tree) SetXattr(name, attr string, data []byte) error {
	f, err := t.openNoFollow(name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return unix.Fsetxattr(int(f.Fd()), attr, data, 0)
}

func (t *Tree) GetXattr(name, attr string) ([]byte, error) {
	f, err := t.openNoFollow(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, xattrMax)
	n, err := unix.Fgetxattr(int(f.Fd()), attr, buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (t *Tree) RemoveXattr(name, attr string) error {
	f, err := t.openNoFollow(name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return unix.Fremovexattr(int(f.Fd()), attr)
}

func (host) SetXattr(name, attr string, data []byte) error {
	return unix.Lsetxattr(name, attr, data, 0)
}

func (host) GetXattr(name, attr string) ([]byte, error) {
	buf := make([]byte, xattrMax)
	n, err := unix.Lgetxattr(name, attr, buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (host) RemoveXattr(name, attr string) error {
	return unix.Lremovexattr(name, attr)
}
