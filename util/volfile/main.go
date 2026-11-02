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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// uploadPrefix starts the name a file is written under until it is
	// complete.
	uploadPrefix = ".om-upload-"

	// copyBuffer is what a transfer writes between two looks at what the
	// others take, which is what a volume may hold over its size until then.
	copyBuffer = 1024 * 1024

	// staleUpload is how long a transfer may go without writing: a client
	// sending nothing is cut long before.
	staleUpload = 15 * time.Minute
)

var (
	// ErrFull is a file not fitting in the size of a volume.
	ErrFull = errors.New("volume full")

	// ErrExists is a file of the name existing, an upload not to replace it.
	ErrExists = errors.New("file exists")
)

type (
	// Info is what a listing says of a file. UploadedBy and UploadedAt are
	// the mark of an upload, empty for a file not uploaded, or on a
	// filesystem not keeping it.
	Info struct {
		Name       string
		Size       int64
		ModifiedAt time.Time
		UploadedBy string
		UploadedAt time.Time
	}

	// Upload says how a file is written: who sends it, and whether it
	// replaces a file of its name.
	Upload struct {
		By      string
		Replace bool
	}

	// mark is what a file uploaded is marked with, in the markAttr extended
	// attribute. A file an application rewrites in place keeps it.
	mark struct {
		By string    `json:"by"`
		At time.Time `json:"at"`
	}
)

// markAttr is the extended attribute marking a file uploaded.
const markAttr = "user.opensvc.upload"

// Exists says a file of the name is at the top of head, whatever it is.
func Exists(head, name string) (bool, error) {
	root, err := os.OpenRoot(head)
	if err != nil {
		return false, err
	}
	defer func() { _ = root.Close() }()
	if _, err := root.Lstat(name); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// IsName says a name is the one of a file at the top of a volume: no path, and
// not what a listing leaves out.
func IsName(s string) bool {
	return s != "" && !strings.HasPrefix(s, ".") && !strings.ContainsAny(s, "/\\\x00") && len(s) <= 255
}

// taken returns what the files of a volume hold but the one of the name, being
// replaced, and the transfer tmp: the other transfers, and what they will be
// renamed to, count. A filesystem bounds what it holds, but not the directory
// of a pool of that type, whose volumes share the one of the node.
func taken(root *os.Root, name, tmp string) (int64, error) {
	d, err := root.Open(".")
	if err != nil {
		return 0, err
	}
	defer func() { _ = d.Close() }()
	entries, err := d.ReadDir(-1)
	if err != nil {
		return 0, err
	}
	var n int64
	for _, e := range entries {
		if e.Name() == name || e.Name() == tmp || !e.Type().IsRegular() {
			continue
		}
		// One gone since the listing, a transfer ending, is under its new name.
		if info, err := e.Info(); err == nil {
			n += info.Size()
		}
	}
	return n, nil
}

// Write writes r to the file of the name in head, the files of head and the
// transfers running with it holding size bytes at most, or as much as r says
// when size is negative. The file is written aside, to appear complete or not,
// and marked uploaded.
//
// What the others take is read again as the transfer goes, not once at its
// start: they run in processes of their own, each of which would otherwise
// count on the same room.
//
// A file of the name existing as the transfer ends is ErrExists, unless the
// upload replaces it: the file comes by a link of its own name, which the
// kernel refuses to an existing one, where a rename would replace it.
func Write(head, name string, r io.Reader, size int64, upload Upload) (err error) {
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
	err = copyWithin(root, f, r, name, tmp, size)
	if err == nil {
		setMark(f, mark{By: upload.By, At: time.Now()})
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if upload.Replace {
		return root.Rename(tmp, name)
	}
	if err = root.Link(tmp, name); errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%w: %s", ErrExists, name)
	} else if err != nil {
		return err
	}
	_ = root.Remove(tmp)
	return nil
}

// copyWithin copies r to f, as long as the size holds what it wrote.
func copyWithin(root *os.Root, f io.Writer, r io.Reader, name, tmp string, size int64) error {
	buf := make([]byte, copyBuffer)
	var written int64
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			written += int64(n)
			if size >= 0 {
				others, err := taken(root, name, tmp)
				if err != nil {
					return err
				}
				if others+written > size {
					return ErrFull
				}
			}
		}
		if errors.Is(rerr, io.EOF) {
			return nil
		} else if rerr != nil {
			return rerr
		}
	}
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
		item := Info{Name: e.Name(), Size: info.Size(), ModifiedAt: info.ModTime()}
		if m, ok := getMark(filepath.Join(head, e.Name())); ok {
			item.UploadedBy, item.UploadedAt = m.By, m.At
		}
		l = append(l, item)
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
