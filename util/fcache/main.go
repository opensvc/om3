// package fcache provide session cache for functions
package fcache

import (
	"errors"
	"github.com/opensvc/fcache"
	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/xsession"
	"path/filepath"
	"strings"
	"time"
)

var (
	maxLockDuration = 30 * time.Second
)

// Output manage output session function cache
func Output(o fcache.Outputter, sig string) (out []byte, err error) {
	return fcache.Output(o, sig, cacheDir(), maxLockDuration, outputLockP)
}

// Purge session function cache
func PurgeCache() error {
	cacheDir := cacheDir()
	if !strings.Contains(cacheDir, "/cache/") {
		return errors.New("unexpected cachedir " + cacheDir)
	}
	return fcache.Purge(cacheDir)
}

func outputLockP(name string) fcache.Locker {
	sessionId := xsession.Id()
	path := filepath.Join(config.Node.Paths.Lock, sessionId+"-out-"+name)
	return flock.New(path, sessionId, fcntllock.New)
}

func cacheDir() string {
	return filepath.Join(config.Node.Paths.Cache, xsession.Id())
}
