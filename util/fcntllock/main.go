package fcntllock

//go:generate mockgen -source=main.go -destination=../mock_fnctllock/main.go

import (
	"context"
	"io"
	"os"
	"syscall"
	"time"
)

type ReadWriteSeekCloser interface {
	io.ReadWriteSeeker
	io.Closer
}

type Locker interface {
	LockContext(context.Context, time.Duration) error
	UnLock() error
	io.ReadWriteSeeker
	io.Closer
}

type P struct{}

type LockProvider interface {
	New(string) Locker
}

// Lock implement fcntl lock features
type Lock struct {
	path string
	ReadWriteSeekCloser
	fd uintptr
}

// New create a new fcntl lock
func (p *P) New(path string) *Lock {
	return &Lock{
		path: path,
	}
}

// TryLock acquires an exclusive write file lock (non blocking)
func (lck *Lock) TryLock() error {
	return lck.lock(false)
}

// Unlock release lock
func (lck Lock) UnLock() (err error) {
	ft := &syscall.Flock_t{
		Start:  0,
		Len:    0,
		Pid:    0,
		Type:   syscall.F_UNLCK,
		Whence: io.SeekStart,
	}
	err = syscall.FcntlFlock(lck.fd, syscall.F_SETLK, ft)
	return
}

// LockContext repeat TryLock with retry delay until succeed or context Done
func (lck *Lock) LockContext(ctx context.Context, retryDelay time.Duration) error {
	return lck.try(ctx, lck.TryLock, retryDelay)
}

func (lck *Lock) lock(blocking bool) (err error) {
	if lck.ReadWriteSeekCloser == nil {
		file, err := os.OpenFile(lck.path, os.O_CREATE|os.O_RDWR|os.O_SYNC, 0666)
		if err != nil {
			return err
		}
		lck.fd = file.Fd()
		lck.ReadWriteSeekCloser = file
	}
	ft := &syscall.Flock_t{
		Start:  0,
		Len:    0,
		Pid:    int32(os.Getpid()),
		Type:   syscall.F_WRLCK,
		Whence: io.SeekStart,
	}
	var cmd int
	if blocking {
		cmd = syscall.F_SETLKW
	} else {
		cmd = syscall.F_SETLK
	}
	if err = syscall.FcntlFlock(lck.fd, cmd, ft); err != nil {
		_ = lck.Close()
		lck.ReadWriteSeekCloser = nil
	}
	return
}

func (lck *Lock) try(ctx context.Context, fn func() error, retryDelay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for {
		if err := fn(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			// context reach end
			return ctx.Err()
		case <-time.After(retryDelay):
			// will try again fn()
		}
	}
}
