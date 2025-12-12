package lock

import (
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/v3/util/xsession"
)

// Func call f() inside lock protection
func Func(lockPath string, timeout time.Duration, intent string, f func() error) error {
	lock := flock.New(lockPath, xsession.ID.String(), fcntllock.New)
	log.Debug().Msgf("Locking %s: %s", intent, lockPath)
	err := lock.Lock(timeout, intent)
	if err != nil {
		return err
	}
	defer func() {
		_ = lock.UnLock()
		log.Debug().Msgf("unLocked %s: %s", intent, lockPath)
	}()
	return f()
}

// Lock tries lock and returns release function to release lock
func Lock(lockPath string, timeout time.Duration, intent string) (func(), error) {
	lock := flock.New(lockPath, xsession.ID.String(), fcntllock.New)
	log.Debug().Msgf("Locking %s: %s", intent, lockPath)
	err := lock.Lock(timeout, intent)
	if err != nil {
		return nil, err
	}
	return func() {
		_ = lock.UnLock()
		log.Debug().Msgf("unLocked %s: %s", intent, lockPath)
	}, nil
}
