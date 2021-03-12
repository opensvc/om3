package object

import (
	"path/filepath"
	"time"

	"opensvc.com/opensvc/util/flock"

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
func (t *Base) Lock(group string, timeout time.Duration, intent string) (*flock.T, error) {
	p := t.LockFile(group)
	log.Debugf("locking %s, timeout %s", p, timeout)
	lock := flock.New(p)
	_, err := lock.Lock(timeout, intent)
	if err != nil {
		return nil, err
	}
	log.Debugf("locked %s", p)
	return lock, nil
}
