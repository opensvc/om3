package object

import (
	"context"
	"path/filepath"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/util/xsession"
)

func (t *core) lockPath(group string) (path string) {
	if group == "" {
		group = "generic"
	}
	path = filepath.Join(t.path.VarDir(), "lock", group)
	return
}

func (t *core) lockAction(ctx context.Context) (func(), error) {
	unlock := func() {}
	props := actioncontext.Props(ctx)
	if !props.MustLock {
		return unlock, nil
	}
	if actioncontext.IsLockDisabled(ctx) {
		// --nolock handling
		return unlock, nil
	}
	p := t.lockPath(props.LockGroup)
	lock := flock.New(p, xsession.ID.String(), fcntllock.New)
	err := lock.Lock(actioncontext.LockTimeout(ctx), props.Name)
	if err != nil {
		return unlock, err
	}
	unlock = func() { _ = lock.UnLock() }

	// the config may have changed since we first read it.
	// ex:
	//  set --kw env.a=a &
	//  set --kw env.b=b
	//
	// These parallel commands end up with either a or b set,
	// because the 2 process load the config cache before locking.
	t.reloadConfig()

	return unlock, nil
}
