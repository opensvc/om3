// Package volfile reads and writes the files of a mounted volume.
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
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"syscall"
	"time"
)

const (
	// uploadPrefix starts the name a file is written under until it is
	// complete, at the top of the volume whatever its directory: the
	// transfers running are found there.
	uploadPrefix = ".om-upload-"

	// copyBuffer is what a transfer writes between two looks at what the
	// others take, which is what a volume may hold over its size until then.
	copyBuffer = 1024 * 1024

	// staleUpload is how long a transfer may go without writing: a client
	// sending nothing is cut long before.
	staleUpload = 15 * time.Minute

	// MaxList is the most entries a listing returns, the first by name.
	MaxList = 1000

	// TypeFile, TypeDir and TypeLink are the types of the entries a listing
	// returns. Sockets, fifos and devices are left out.
	TypeFile = "file"
	TypeDir  = "dir"
	TypeLink = "link"
)

var (
	// ErrFull is a file not fitting in the size of a volume.
	ErrFull = errors.New("volume full")

	// ErrExists is a file of the name existing, an upload not to replace it,
	// or a directory to make.
	ErrExists = errors.New("file exists")

	// ErrInvalidDir is a directory leading out of the volume, or hidden.
	ErrInvalidDir = errors.New("invalid directory")

	// ErrNoDir is a directory not existing.
	ErrNoDir = errors.New("no such directory")

	// ErrNotDir is a directory that is something else.
	ErrNotDir = errors.New("not a directory")

	// ErrNotEmpty is a directory to remove holding something.
	ErrNotEmpty = errors.New("directory not empty")

	// ErrNotFile is an entry to read that is no regular file: a link is
	// not followed.
	ErrNotFile = errors.New("not a file")

	// ErrInstalled is a file a datastore install writes, an upload not to
	// replace, replace or not.
	ErrInstalled = errors.New("installed file")
)

type (
	// Info is what a listing says of an entry. UploadedBy and UploadedAt
	// are the mark of an upload, empty for a file not uploaded, or on a
	// filesystem not keeping it, and Installed the mark of a datastore
	// install. Target is what a link says, never followed.
	Info struct {
		Name       string
		Type       string
		Size       int64
		ModifiedAt time.Time
		UploadedBy string
		UploadedAt time.Time
		Installed  *InstallMark
		Target     string
	}

	// Listing is the entries of a directory, the first MaxList by name, and
	// whether there are more.
	Listing struct {
		Items     []Info
		Truncated bool
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

const (
	// UploadAttr is the extended attribute marking a file uploaded.
	UploadAttr = "user.opensvc.upload"

	// InstallAttr is the extended attribute marking a file a datastore
	// install writes, an InstallMark.
	InstallAttr = "user.opensvc.install"
)

// InstallMark is what a file a datastore install writes is marked with: the
// datastore, its key, and when. An upload does not replace such a file, which
// the next install would write again: the key is what to change.
type InstallMark struct {
	From string    `json:"from"`
	Key  string    `json:"key"`
	At   time.Time `json:"at"`
}

// installedError says why an upload does not replace an installed file.
func installedError(name string, m InstallMark) error {
	return fmt.Errorf("%w: %s is installed from %s key %s: change the key", ErrInstalled, name, m.From, m.Key)
}

// Installed returns the mark of the install of the entry of the name in the
// directory, ok false when it has none.
func Installed(head, dir, name string) (m InstallMark, ok bool, err error) {
	root, err := openRoot(head, dir)
	if err != nil {
		return m, false, err
	}
	defer func() { _ = root.Close() }()
	m, ok = installMarkOf(root, dir, name)
	return m, ok, nil
}

func installMarkOf(root *os.Root, dir, name string) (InstallMark, bool) {
	d, err := root.Open(dir)
	if err != nil {
		return InstallMark{}, false
	}
	defer func() { _ = d.Close() }()
	return getInstallMark(d, name)
}

// IsName says a name is the one of an entry of a directory: no path, and not
// what a listing leaves out.
func IsName(s string) bool {
	return s != "" && !strings.HasPrefix(s, ".") && !strings.ContainsAny(s, "/\\\x00") && len(s) <= 255
}

// CleanDir returns a directory of a volume as the functions here take it, "."
// for the top: relative to the head, with no component leading up, nor hidden,
// which a listing leaves out.
func CleanDir(s string) (string, error) {
	if s == "" {
		return ".", nil
	}
	if strings.HasPrefix(s, "/") || strings.ContainsAny(s, "\\\x00") {
		return "", fmt.Errorf("%w: %q", ErrInvalidDir, s)
	}
	for _, e := range strings.Split(s, "/") {
		if e == ".." || (e != "." && strings.HasPrefix(e, ".")) {
			return "", fmt.Errorf("%w: %q", ErrInvalidDir, s)
		}
	}
	return path.Clean(s), nil
}

// openRoot opens the volume, and checks the directory of it is one.
func openRoot(head, dir string) (*os.Root, error) {
	root, err := os.OpenRoot(head)
	if err != nil {
		return nil, err
	}
	if err := checkDir(root, dir); err != nil {
		_ = root.Close()
		return nil, err
	}
	return root, nil
}

func checkDir(root *os.Root, dir string) error {
	info, err := root.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %s", ErrNoDir, dir)
	case errors.Is(err, syscall.ENOTDIR):
		return fmt.Errorf("%w: %s", ErrNotDir, dir)
	case err != nil:
		// A link leading out of the volume, which the root refuses.
		return fmt.Errorf("%w: %s: %w", ErrInvalidDir, dir, err)
	case !info.IsDir():
		return fmt.Errorf("%w: %s", ErrNotDir, dir)
	}
	return nil
}

// Exists says an entry of the name is in the directory, whatever it is.
func Exists(head, dir, name string) (bool, error) {
	root, err := openRoot(head, dir)
	if err != nil {
		return false, err
	}
	defer func() { _ = root.Close() }()
	if _, err := root.Lstat(path.Join(dir, name)); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// Stat returns what the entry of the name in the directory is, not following
// a link, ErrNotFile when it is no regular file.
func Stat(head, dir, name string) (os.FileInfo, error) {
	root, err := openRoot(head, dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return statFile(root, path.Join(dir, name), name)
}

func statFile(root *os.Root, p, name string) (os.FileInfo, error) {
	info, err := root.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s", ErrNotFile, name)
	}
	return info, nil
}

// Open opens the file of the name in the directory for reading, a link not
// followed: the file opened is the one Lstat said was a regular file.
func Open(head, dir, name string) (*os.File, error) {
	root, err := openRoot(head, dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	p := path.Join(dir, name)
	before, err := statFile(root, p, name)
	if err != nil {
		return nil, err
	}
	f, err := root.Open(p)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		_ = f.Close()
		// Replaced between the two, by a link say.
		return nil, fmt.Errorf("%w: %s", ErrNotFile, name)
	}
	return f, nil
}

// Mkdir makes the directory of the name in the directory, and no parent.
func Mkdir(head, dir, name string) error {
	root, err := openRoot(head, dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := root.Mkdir(path.Join(dir, name), 0o755); errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%w: %s", ErrExists, name)
	} else if err != nil {
		return err
	}
	return nil
}

// Remove removes the entry of the name in the directory: a file, a link but
// not what it leads to, or an empty directory.
func Remove(head, dir, name string) error {
	root, err := openRoot(head, dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	err = root.Remove(path.Join(dir, name))
	if errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST) {
		return fmt.Errorf("%w: %s", ErrNotEmpty, name)
	}
	return err
}

// Write writes r to the file of the name in the directory, the files of the
// volume and the transfers running with it holding size bytes at most, or as
// much as r says when size is negative. The file is written aside, to appear
// complete or not, and marked uploaded.
//
// What the volume holds is read as the transfer begins and as it ends, and
// what the other transfers write as it goes: they run in processes of their
// own, each of which would otherwise count on the same room.
//
// A file of the name existing as the transfer ends is ErrExists, unless the
// upload replaces it: the file comes by a link of its own name, which the
// kernel refuses to an existing one, where a rename would replace it.
func Write(head, dir, name string, r io.Reader, size int64, upload Upload) (err error) {
	root, err := openRoot(head, dir)
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
	p := path.Join(dir, name)
	var held int64
	if size >= 0 {
		if held, err = used(root, p); err != nil {
			return err
		}
	}
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = root.Remove(tmp)
		}
	}()
	err = copyWithin(root, f, r, held, tmp, size)
	if err == nil && size >= 0 {
		err = fitsAtEnd(root, p, tmp, size)
	}
	if err == nil {
		setMark(f, mark{By: upload.By, At: time.Now()})
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	// An install of a datastore writes the file: changing it is changing
	// the key, which an upload does not.
	if m, ok := installMarkOf(root, dir, name); ok {
		return installedError(name, m)
	}
	if upload.Replace {
		return root.Rename(tmp, p)
	}
	if err = root.Link(tmp, p); errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%w: %s", ErrExists, name)
	} else if err != nil {
		return err
	}
	_ = root.Remove(tmp)
	return nil
}

// fitsAtEnd says what the volume holds now, the files the other transfers
// completed meanwhile included, leaves room for the file written, tmp. Of two
// transfers ending at once, each sees what the other wrote, as a file of its
// name or its tmp.
func fitsAtEnd(root *os.Root, p, tmp string, size int64) error {
	held, err := used(root, p)
	if err != nil {
		return err
	}
	others, err := transfers(root, tmp)
	if err != nil {
		return err
	}
	info, err := root.Lstat(tmp)
	if err != nil {
		return err
	}
	if held+others+info.Size() > size {
		return ErrFull
	}
	return nil
}

// used returns what the files of the volume hold, but the one at p, to be
// replaced, and the transfers running, counted as they go.
func used(root *os.Root, p string) (int64, error) {
	var n int64
	err := fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			// What can not be read is not counted, rather than refuse
			// all uploads.
			return nil
		case name == p, !d.Type().IsRegular():
			return nil
		case !strings.Contains(name, "/") && strings.HasPrefix(name, uploadPrefix):
			return nil
		}
		if info, err := d.Info(); err == nil {
			n += info.Size()
		}
		return nil
	})
	return n, err
}

// transfers returns what the transfers running write, but tmp.
func transfers(root *os.Root, tmp string) (int64, error) {
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
		if e.Name() == tmp || !strings.HasPrefix(e.Name(), uploadPrefix) || !e.Type().IsRegular() {
			continue
		}
		// One gone since the listing, a transfer ending, is counted by
		// what it wrote as the others began.
		if info, err := e.Info(); err == nil {
			n += info.Size()
		}
	}
	return n, nil
}

// copyWithin copies r to f, as long as the size holds it, what the volume held
// and what the other transfers write.
func copyWithin(root *os.Root, f io.Writer, r io.Reader, held int64, tmp string, size int64) error {
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
				others, err := transfers(root, tmp)
				if err != nil {
					return err
				}
				if held+others+written > size {
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

// List returns the entries of the directory, the hidden ones left out.
func List(head, dir string) (Listing, error) {
	root, err := openRoot(head, dir)
	if err != nil {
		return Listing{}, err
	}
	defer func() { _ = root.Close() }()
	d, err := root.Open(dir)
	if err != nil {
		return Listing{}, err
	}
	defer func() { _ = d.Close() }()
	entries, err := d.ReadDir(-1)
	if err != nil {
		return Listing{}, err
	}
	entries = slices.DeleteFunc(entries, func(e fs.DirEntry) bool {
		t := e.Type()
		return !IsName(e.Name()) || !(t.IsRegular() || t.IsDir() || t&fs.ModeSymlink != 0)
	})
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	listing := Listing{Items: make([]Info, 0, min(len(entries), MaxList))}
	if len(entries) > MaxList {
		entries, listing.Truncated = entries[:MaxList], true
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		item := Info{Name: e.Name(), Size: info.Size(), ModifiedAt: info.ModTime()}
		switch t := e.Type(); {
		case t.IsDir():
			item.Type, item.Size = TypeDir, 0
		case t&fs.ModeSymlink != 0:
			item.Type = TypeLink
			item.Target, _ = root.Readlink(path.Join(dir, e.Name()))
		default:
			item.Type = TypeFile
			if m, ok := getMark(d, e.Name()); ok {
				item.UploadedBy, item.UploadedAt = m.By, m.At
			}
			if m, ok := getInstallMark(d, e.Name()); ok {
				item.Installed = &m
			}
		}
		listing.Items = append(listing.Items, item)
	}
	return listing, nil
}
