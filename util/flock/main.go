package flock

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/fcntllock"
	"os"
	"time"
)

type (
	locker interface {
		LockContext(context.Context, time.Duration) error
		UnLock() error
		io.ReadWriteSeeker
		io.Closer
	}

	// T wraps flock and dumps a json data in the lock file
	// hinting about what holds the lock.
	T struct {
		locker
		Path string
	}

	meta struct {
		PID       int    `json:"pid"`
		Intent    string `json:"intent"`
		SessionID string `json:"session_id"`
	}
)

var truncate = os.Truncate
var remove = os.Remove

type lockProvider interface {
	New(string) fcntllock.Locker
}

// New allocate a file lock struct.
func NewCustomLock(p string, prov lockProvider) *T {
	return &T{
		locker: prov.New(p),
		Path:   p,
	}
}

// New allocate a file lock struct.
func New(p string) *T {
	prov := &fcntllock.P{}
	return &T{
		locker: prov.New(p),
		Path:   p,
	}
}

var retryInterval = 500 * time.Millisecond
var getSessionId = func() string { return config.SessionID }

//
// Lock acquires an exclusive file lock on the file and write a json
// formatted structure hinting who holds the lock and with what
// intention.
//
func (t *T) Lock(timeout time.Duration, intent string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err = t.LockContext(ctx, retryInterval)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return errors.New("lock timeout exceeded")
		}
		return
	}
	err = writeMeta(t, intent)
	return
}

func writeMeta(w io.Writer, intent string) error {
	m := meta{
		PID:       os.Getpid(),
		Intent:    intent,
		SessionID: getSessionId(),
	}
	enc := json.NewEncoder(w)
	return enc.Encode(m)
}

// Unlock releases the file lock acquired by Lock.
func (t *T) UnLock() error {
	_ = truncate(t.Path, 0)
	_ = remove(t.Path)
	return t.locker.UnLock()
}
