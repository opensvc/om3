package object

import (
	"path/filepath"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"opensvc.com/opensvc/util/xsession"
)

func (t *Base) lockPath(group string) (path string) {
	if group == "" {
		group = "generic"
	}
	path = filepath.Join(t.VarDir(), "lock", group)
	return
}

//
// Lock acquires the action lock.
//
// A custom lock group can be specified to prevent parallel run of a subset
// of object actions.
//
func (t *Base) Lock(group string, timeout time.Duration, intent string) (*flock.T, error) {
	p := t.lockPath(group)
	t.log.Debug().Msgf("locking %s, timeout %s", p, timeout)
	lock := flock.New(p, xsession.Id(), fcntllock.New)
	err := lock.Lock(timeout, intent)
	if err != nil {
		return nil, err
	}
	t.log.Debug().Msgf("locked %s", p)
	return lock, nil
}

func (t *Base) lockedAction(group string, timeout time.Duration, intent string, f func() error) error {
	p := t.lockPath(group)
	lock := flock.New(p, xsession.Id(), fcntllock.New)
	err := lock.Lock(timeout, intent)
	if err != nil {
		return err
	}
	defer func() { _ = lock.UnLock() }()
	return f()
}
