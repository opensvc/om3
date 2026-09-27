package lock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Exclusive takes an exclusive lock on the file at path, creating it and its
// directory, and returns the function releasing it. It waits for the lock
// until ctx is done.
//
// It is for the short sections that must not interleave with the same
// section run by another process or another goroutine: the lock is held by
// the open file description, which every call opens anew, so it excludes the
// goroutines of this process as it excludes the other processes. A lock held
// by the process, as a fcntl lock is, would let every goroutine of the daemon
// in at once.
//
// A holder dying releases the lock with its descriptors, so a crash leaves
// nothing to clean up and nobody to wait for.
//
// The file is never removed. Removing a lock file on release lets a waiter
// that opened it before the removal and a newcomer that creates it after both
// hold the lock, on two files of the same name.
func Exclusive(ctx context.Context, path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		f, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	}
	if err != nil {
		return nil, err
	}
	delay := time.Millisecond
	for {
		acquired, err := tryExclusive(f)
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("lock %s: %w", path, err)
		}
		if acquired {
			return func() { _ = f.Close() }, nil
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, fmt.Errorf("lock %s: %w", path, ctx.Err())
		case <-time.After(delay):
		}
		if delay < 50*time.Millisecond {
			delay *= 2
		}
	}
}
