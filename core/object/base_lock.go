package object

import (
	"time"

	"opensvc.com/opensvc/util/flock"
)

// lockName is the lock name of the file to use as an action lock.
func (t *Base) lockName(group string) string {
	p := "lock.generic"
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
	p := t.lockName(group)
	t.log.Debug().Msgf("locking %s, timeout %s", p, timeout)
	lock := flock.New(p)
	err := lock.Lock(timeout, intent)
	if err != nil {
		return nil, err
	}
	t.log.Debug().Msgf("locked %s", p)
	return lock, nil
}

func (t *Base) lockedAction(group string, timeout time.Duration, intent string, f func() error) error {
	p := t.lockName(group)
	lck := flock.New(p)
	err := lck.Lock(timeout, intent)
	if err != nil {
		return err
	}
	defer func() { _ = lck.UnLock() }()
	return f()
}
