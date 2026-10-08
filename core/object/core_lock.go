package object

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/util/key"
	ulock "github.com/opensvc/om3/v3/util/lock"
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
	lock := flock.New(p, xsession.SessionID().String(), fcntllock.New)
	timeout := actioncontext.LockTimeout(ctx)
	syncHolder := t.syncHoldingLock(ctx, lock)
	if syncHolder != nil {
		// A stop waits for a sync as long as an orchestrated one
		// does, not the time a lock is usually waited for, unless the
		// user said how long to wait.
		if d := t.waitSyncsTimeout(); d > timeout && !actioncontext.IsLockTimeoutSet(ctx) {
			timeout = d
		}
		t.log.Infof("wait for the %s of the syncs (pid %d, running for %s) to end, %s at most: --interrupt-syncs ends them instead",
			syncHolder.Intent, syncHolder.PID, time.Since(syncHolder.At).Round(time.Second), timeout)
	}
	err := lock.Lock(timeout, props.Name)
	if err != nil {
		err = ulock.Explain(lock, timeout, err)
		if syncHolder != nil {
			return unlock, fmt.Errorf("the syncs still hold the object lock after %s, --interrupt-syncs ends them: %w", timeout, err)
		}
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

// lockStatus takes the lock every status evaluation of the object holds,
// whatever it is part of.
//
// The status evaluation closing an action runs under the lock of the action,
// which a status refresh does not take: a "status -m" refresh running beside
// the stop of a monitor action read the status of before the stop, and posted
// it stamped after the status the stop closed on, which the daemon dropped as
// the older. The instance stayed up to the daemon, and to the peers waiting
// for it to be down to take over, until the next refresh.
func (t *core) lockStatus(ctx context.Context) (func(), error) {
	if actioncontext.IsLockDisabled(ctx) {
		return func() {}, nil
	}
	lock := flock.New(t.lockPath(actioncontext.Status.LockGroup), xsession.SessionID().String(), fcntllock.New)
	timeout := actioncontext.LockTimeout(ctx)
	if err := lock.Lock(timeout, actioncontext.Status.Name); err != nil {
		return func() {}, ulock.Explain(lock, timeout, err)
	}
	t.reloadConfig()
	return func() { _ = lock.UnLock() }, nil
}

// syncHoldingLock is the holder of the object lock when it is a sync and the
// action is a stop, which would otherwise wait the time a lock is usually
// waited for, and fail with nothing saying why.
func (t *core) syncHoldingLock(ctx context.Context, lock *flock.T) *flock.Meta {
	if actioncontext.Props(ctx).Name != actioncontext.Stop.Name {
		return nil
	}
	holder, err := lock.Probe()
	if err != nil || holder.PID == 0 {
		return nil
	}
	switch holder.Intent {
	case actioncontext.SyncUpdate.Name, actioncontext.SyncFull.Name:
		return &holder
	default:
		return nil
	}
}

// waitSyncsTimeout is how long a stop waits for the syncs running:
// DEFAULT.wait_syncs_timeout.
func (t *core) waitSyncsTimeout() time.Duration {
	if d := t.config.GetDuration(key.New("DEFAULT", "wait_syncs_timeout")); d != nil {
		return *d
	}
	return 10 * time.Minute
}
