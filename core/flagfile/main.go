// Package flagfile handles the flag files an object keeps in its var
// directory, where the presence of the file is the flag and its modification
// time is when the flag was raised.
//
// A flag file is what makes a decision survive a daemon restart or a reboot:
// the frozen flag says the daemon may not act on the instance, the stopped
// flag says the daemon may not start it back on its own.
package flagfile

import (
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/util/file"
)

// Set raises the flag, creating the var directory if needed. Setting a flag
// already raised leaves its time alone: the flag says when the decision was
// made, and it was made the first time.
func Set(p string) error {
	if file.Exists(p) {
		return nil
	}
	d := filepath.Dir(p)
	if !file.Exists(d) {
		if err := os.MkdirAll(d, os.ModePerm); err != nil {
			return err
		}
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// Unset lowers the flag, and syncs the directory so the removal survives a
// crash.
func Unset(p string) error {
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
