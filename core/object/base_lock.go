package object

import (
	"path/filepath"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"opensvc.com/opensvc/util/xsession"
)

func (t *Base) lockPath(group string) (path string) {
	if group == "" {
		group = "generic"
	}
	path = filepath.Join(VarDir(t.Path), "lock", group)
	return
}

func (t *Base) lockedAction(group string, options OptsLock, intent string, f func() error) error {
	if options.Disable {
		// --nolock handling
		return nil
	}
	p := t.lockPath(group)
	lock := flock.New(p, xsession.ID, fcntllock.New)
	err := lock.Lock(options.Timeout, intent)
	if err != nil {
		return err
	}
	defer func() { _ = lock.UnLock() }()

	// the config may have changed since we first read it.
	// ex:
	//  set --kw env.a=a &
	//  set --kw env.b=b
	//
	// These parallel commands end up with either a or b set,
	// because the 2 process load the config cache before locking.
	t.reloadConfig()

	return f()
}
