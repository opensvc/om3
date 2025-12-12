// Package fcache provide session cache for functions
package fcache

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/opensvc/fcache"
	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/xsession"
)

var (
	maxLockDuration = 30 * time.Second
	isUsed          atomic.Bool
)

// Output manage output session function cache
func Output(o fcache.Outputter, sig string) (out []byte, err error) {
	isUsed.Store(true)
	return fcache.Output(o, sig, cacheDir(), maxLockDuration, outputLockP)
}

// Clear removes the current cached output
func Clear(sig string) error {
	return fcache.Clear(sig, cacheDir(), maxLockDuration, outputLockP)
}

// PurgeCache purges session cache
func PurgeCache() error {
	if !isUsed.Load() {
		return nil
	}
	isUsed.Store(false)
	cacheDir := cacheDir()
	if !strings.Contains(cacheDir, "/cache/") {
		return fmt.Errorf("unexpected cachedir %s", cacheDir)
	}
	return fcache.Purge(cacheDir)
}

func outputLockP(name string) fcache.Locker {
	sessionID := xsession.ID
	path := filepath.Join(rawconfig.Paths.Lock, sessionID.String()+"-out-"+name)
	return flock.New(path, sessionID.String(), fcntllock.New)
}

func cacheDir() string {
	return filepath.Join(rawconfig.Paths.Cache, xsession.ID.String())
}
