// package fcache provide session cache for functions
package fcache

import (
	"github.com/opensvc/fcache"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/flock"
	"opensvc.com/opensvc/util/xsession"
	"path/filepath"
	"time"
)

var (
	sessionId       = xsession.Id()
	cacheDir        = filepath.Join(config.Node.Paths.Var, "cache", sessionId)
	maxLockDuration = 30 * time.Second
	lockerP         = flock.New
)

// Output manage output session function cache
func Output(o fcache.Outputter, sig string) (out []byte, err error) {
	return fcache.Output(o, sig, cacheDir, maxLockDuration, outputLockP)
}

// Purge session function cache
func PurgeCache() error {
	return fcache.Purge(cacheDir)
}

func outputLockP(name string) fcache.Locker {
	return lockerP(sessionId + "-out-" + name)
}
