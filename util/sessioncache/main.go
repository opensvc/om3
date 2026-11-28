// Package sessioncache provide session cache for functions
//
// The processes of a session share its cache: the commands an action runs,
// and the actions a multi-object command runs on a node, read the outputs the
// others cached. A process using the cache holds a shared fcntl lock on the
// session's users file while it runs, and the cache is purged by the last one
// exiting only: one exiting first does not remove the cache under the others.
package sessioncache

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/opensvc/fcache"
	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/xsession"
)

var (
	maxLockDuration = 30 * time.Second

	// users is the lock of the users file of the session cache this
	// process holds a shared lock on, nil when it does not use the cache.
	users   *fcntllock.Lock
	usersMu sync.Mutex
)

// usersFileName is the file of the session cache directory the processes
// using it hold a shared lock on.
const usersFileName = ".users"

// Output manage output session function cache
//
// A cache file removed while it is read or written, as by a process of the
// session purging it, is a cache miss: the output is the one of the command.
func Output(o fcache.Outputter, sig string) (out []byte, err error) {
	if err := use(); err != nil {
		return o.Output()
	}
	out, err = fcache.Output(o, sig, cacheDir(), maxLockDuration, outputLockP)
	if errors.Is(err, fs.ErrNotExist) {
		return o.Output()
	}
	return out, err
}

// Clear removes the current cached output
func Clear(sig string) error {
	return fcache.Clear(sig, cacheDir(), maxLockDuration, outputLockP)
}

// use takes a shared lock on the users file of the session cache, once per
// process until it purges, so the cache is not purged by another process of
// the session while this one uses it.
//
// The lock is a fcntl one, held by the process: the users file is opened
// once, by this lock, and is not to be opened elsewhere in the process, where
// closing it would release the lock.
func use() error {
	usersMu.Lock()
	defer usersMu.Unlock()
	if users != nil {
		return nil
	}
	dir := cacheDir()
	path := filepath.Join(dir, usersFileName)
	for i := 0; i < 5; i++ {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		l, ok := fcntllock.New(path).(*fcntllock.Lock)
		if !ok {
			return fmt.Errorf("lock the users of the session cache %s: not a fcntl lock", dir)
		}
		ctx, cancel := context.WithTimeout(context.Background(), maxLockDuration)
		err := l.RLockContext(ctx, 10*time.Millisecond)
		cancel()
		if errors.Is(err, fs.ErrNotExist) {
			// The directory purged meanwhile.
			continue
		} else if err != nil {
			return err
		}
		// The file locked is the one of the path still: a process
		// purging the cache between the open and the lock removed it,
		// and a lock on a file removed would not hold the next purge.
		if f, ok := l.ReadWriteSeekCloser.(*os.File); ok {
			fi, err1 := f.Stat()
			pi, err2 := os.Stat(path)
			if err1 == nil && err2 == nil && os.SameFile(fi, pi) {
				users = l
				return nil
			}
		}
		_ = l.Close()
	}
	return fmt.Errorf("lock the users of the session cache %s: the cache is purged repeatedly", dir)
}

// PurgeCache purges the session cache when no other process of the session
// uses it, and releases the use of this process. The last process using it
// purges it.
func PurgeCache() error {
	usersMu.Lock()
	defer usersMu.Unlock()
	if users == nil {
		return nil
	}
	l := users
	users = nil
	defer func() { _ = l.Close() }()
	cacheDir := cacheDir()
	if !strings.Contains(cacheDir, "/cache/") {
		return fmt.Errorf("unexpected cachedir %s", cacheDir)
	}
	// The shared lock converted to an exclusive one: refused while another
	// process of the session holds a shared lock, which purges the cache,
	// the last one exiting.
	if err := l.TryLock(); err != nil {
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EACCES) {
			return nil
		}
		return err
	}
	return fcache.Purge(cacheDir)
}

func outputLockP(name string) fcache.Locker {
	sessionID := xsession.SessionID().String()
	path := filepath.Join(rawconfig.Paths.Lock, sessionID+"-out-"+name)
	return flock.New(path, sessionID, fcntllock.New)
}

func cacheDir() string {
	sessionID := xsession.SessionID().String()
	return filepath.Join(rawconfig.Paths.Cache, sessionID)
}
