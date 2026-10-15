// Package flagfile handles the flag files an object keeps in its var
// directory, where the presence of the file is the flag and its modification
// time is when the flag was raised.
//
// A flag file is what makes a decision survive a daemon restart or a reboot:
// the frozen flag says the daemon may not act on the instance, the stopped
// flag says the daemon may not start it back on its own.
package flagfile

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/lock"
)

// lockTimeout bounds the wait for the lock of a flag, which its holders keep
// for a create or a removal.
var lockTimeout = 10 * time.Second

// lockFlag takes the lock of the flag p, which every change of the flag
// takes, so a check of what the flag is and the change made on it are one
// step. The lock file is beside the flag, hidden, and never removed.
func lockFlag(p string) (func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()
	return lock.Exclusive(ctx, filepath.Join(filepath.Dir(p), "."+filepath.Base(p)+".lock"))
}

// mkdir creates the directory of the flag p if needed.
func mkdir(p string) error {
	d := filepath.Dir(p)
	if file.Exists(d) {
		return nil
	}
	return os.MkdirAll(d, os.ModePerm)
}

// Set raises the flag, creating the var directory if needed. Setting a flag
// already raised leaves its time alone: the flag says when the decision was
// made, and it was made the first time.
func Set(p string) error {
	return SetWith(p, nil)
}

// SetWith raises the flag like Set, with b as the content of the file, which
// says more about the decision than its time. Setting a flag already raised
// leaves its content alone too, as the decision it describes.
func SetWith(p string, b []byte) error {
	if file.Exists(p) {
		return nil
	}
	if err := mkdir(p); err != nil {
		return err
	}
	unlock, err := lockFlag(p)
	if err != nil {
		return err
	}
	defer unlock()
	if file.Exists(p) {
		return nil
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(b) > 0 {
		if _, err := f.Write(b); err != nil {
			return err
		}
	}
	return f.Sync()
}

// Raised is a flag a call of Raise raised, until it is kept or lowered.
//
// It holds the file it created open, so the file is told apart from one
// created at the same path after it was removed, which may take its inode
// when it is free.
type Raised struct {
	p string
	f *os.File
}

// Raise raises the flag as Set does, for a caller that may have to take it
// away again, as a write that raised it for itself and did not land. It
// returns nil when the flag was raised already, as it is not the caller's
// to take away. The flag returned, raised even with an error, as a file
// created and not synced, is to be kept or lowered.
func Raise(p string) (*Raised, error) {
	if err := mkdir(p); err != nil {
		return nil, err
	}
	unlock, err := lockFlag(p)
	if err != nil {
		return nil, err
	}
	defer unlock()
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if errors.Is(err, fs.ErrExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &Raised{p: p, f: f}, f.Sync()
}

// Keep leaves the flag raised.
func (t *Raised) Keep() error {
	return t.f.Close()
}

// Lower takes the flag away while it is the file Raise created: a flag
// another raised after it was removed is left alone. The check and the
// removal are made under the lock of the flag, so no other change of the
// flag lands between the two.
func (t *Raised) Lower() error {
	defer t.f.Close()
	unlock, err := lockFlag(t.p)
	if err != nil {
		return err
	}
	defer unlock()
	created, err := t.f.Stat()
	if err != nil {
		return err
	}
	current, err := os.Stat(t.p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if !os.SameFile(created, current) {
		return nil
	}
	return unset(t.p)
}

// Content returns the content the flag was raised with, empty when it was
// raised with none or is not raised.
func Content(p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	return b
}

// Unset lowers the flag, and syncs the directory so the removal survives a
// crash.
func Unset(p string) error {
	if !file.Exists(p) {
		return nil
	}
	unlock, err := lockFlag(p)
	if err != nil {
		return err
	}
	defer unlock()
	return unset(p)
}

// unset lowers the flag, under its lock.
func unset(p string) error {
	if !file.Exists(p) {
		return nil
	}
	if err := os.Remove(p); err != nil {
		return err
	}
	dir := filepath.Dir(p)
	fd, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer fd.Close()
	return fd.Sync()
}

// At returns the time the flag was raised, or the zero time when it is not.
func At(p string) time.Time {
	fi, err := os.Stat(p)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}
