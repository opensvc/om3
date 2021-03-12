package object

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	log "github.com/sirupsen/logrus"
)

// LockFile is the path of the file to use as an action lock.
func (t *Base) LockFile(group string) string {
	p := filepath.Join(t.varDir(), "lock.generic")
	if group != "" {
		p += "." + group
	}
	return p
}

//
// Lock acquires the action lock.
//
// A custom lock group can be specified to prevent parallel run of a subset
// of object actions.
//
func (t *Base) Lock(group string, timeout time.Duration) error {
	p := t.LockFile(group)
	log.Debugf("locking %s, timeout %s", p, timeout)
	f := flock.New(p)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := f.TryLockContext(ctx, 500*time.Millisecond)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return errors.New("lock timeout exceeded")
		}
		return err
	}
	log.Debugf("locked %s", p)
	return nil
}

//
// Unlock releases the action lock.
//
// A custom lock group can be specified to prevent parallel run of a subset
// of object actions.
//
func (t *Base) Unlock(group string) error {
	p := t.LockFile(group)
	log.Debugf("unlock %s", p)
	f := flock.New(p)
	f.Unlock()
	return nil
}
