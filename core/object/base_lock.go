package object

import (
	"path/filepath"
	"time"

	"opensvc.com/opensvc/util/flock"
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
func (t *Base) Lock(group string, timeout time.Duration, intent string) (*flock.T, error) {
	p := t.LockFile(group)
	t.log.Debug().Msgf("locking %s, timeout %s", p, timeout)
	lock := flock.New(p)
	_, err := lock.Lock(timeout, intent)
	if err != nil {
		return nil, err
	}
	t.log.Debug().Msgf("locked %s", p)
	return lock, nil
}

func (t *Base) lockedAction(group string, timeout time.Duration, intent string, f func() error) error {
	lock, err := t.Lock(group, timeout, intent)
	if err != nil {
		return err
	}
	defer lock.Unlock()
	return f()
}
