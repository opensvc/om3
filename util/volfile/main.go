// Package volfile reads and writes the files at the top of a mounted volume.
//
// What a volume holds is its user's, a symbolic link to a file of the node
// included, and this code runs as root: the root of the volume confines what
// is opened, so nothing here leads out of it.
package volfile

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
	"time"
)

const (
	// uploadPrefix starts the name a file is written under until it is
	// complete.
	uploadPrefix = ".om-upload-"

	// staleUpload is how long a transfer may go without writing: a client
	// sending nothing is cut long before.
	staleUpload = 15 * time.Minute
)

// ErrFull is a file not fitting in the size of a volume.
var ErrFull = errors.New("volume full")

// Info is what a listing says of a file.
type Info struct {
	Name       string
	Size       int64
	ModifiedAt time.Time
}

// IsName says a name is the one of a file at the top of a volume: no path, and
// not what a listing leaves out.
func IsName(s string) bool {
	return s != "" && !strings.HasPrefix(s, ".") && !strings.ContainsAny(s, "/\\\x00") && len(s) <= 255
}

// Room returns what a file of the name may take in a volume of the size: what
// the other files leave, the one of that name being replaced. A filesystem
// bounds what it holds, but not the directory of a pool of that type, whose
// volumes share the one of the node.
func Room(head, name string, size int64) (int64, error) {
	l, err := List(head)
	if err != nil {
		return 0, err
	}
	for _, f := range l {
		if f.Name != name {
			size -= f.Size
		}
	}
	return max(size, 0), nil
}

// Write writes r to the file of the name in head, limit bytes at most, none
// when negative. The file is written aside, to appear complete or not.
func Write(head, name string, r io.Reader, limit int64) (err error) {
	root, err := os.OpenRoot(head)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	sweep(root)
	// A name of its own: transfers run in processes of their own, two of a
	// same file at once included, the last one ending being the one kept.
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	tmp := uploadPrefix + hex.EncodeToString(suffix)
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = root.Remove(tmp)
		}
	}()
	if limit >= 0 {
		r = io.LimitReader(r, limit+1)
	}
	n, err := io.Copy(f, r)
	if err == nil && limit >= 0 && n > limit {
		err = ErrFull
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return root.Rename(tmp, name)
}

// sweep removes what the transfers killed left: a listing does not show it, so
// the size of a volume would not count it.
func sweep(root *os.Root) {
	d, err := root.Open(".")
	if err != nil {
		return
	}
	defer func() { _ = d.Close() }()
	entries, err := d.ReadDir(-1)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), uploadPrefix) {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > staleUpload {
			_ = root.Remove(e.Name())
		}
	}
}

// List returns the files at the top of head.
func List(head string) ([]Info, error) {
	root, err := os.OpenRoot(head)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	d, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = d.Close() }()
	entries, err := d.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	l := make([]Info, 0, len(entries))
	for _, e := range entries {
		if !e.Type().IsRegular() || !IsName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		l = append(l, Info{Name: e.Name(), Size: info.Size(), ModifiedAt: info.ModTime()})
	}
	return l, nil
}

// Remove removes the file of the name in head.
func Remove(head, name string) error {
	root, err := os.OpenRoot(head)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return root.Remove(name)
}
