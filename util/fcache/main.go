// Package fcache provide session cache for functions
package fcache

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/fcache"
	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/xsession"
)

var (
	maxLockDuration = 30 * time.Second
)

// Output manage output session function cache
func Output(o fcache.Outputter, sig string) (out []byte, err error) {
	return fcache.Output(o, sig, cacheDir(), maxLockDuration, outputLockP)
}

// Clear removes the current cached output
func Clear(sig string) error {
	return fcache.Clear(sig, cacheDir(), maxLockDuration, outputLockP)
}

// PurgeCache purges session cache
func PurgeCache() error {
	cacheDir := cacheDir()
	if !strings.Contains(cacheDir, "/cache/") {
		return fmt.Errorf("unexpected cachedir %s", cacheDir)
	}
	return fcache.Purge(cacheDir)
}

func outputLockP(name string) fcache.Locker {
	sessionId := xsession.ID
	path := filepath.Join(rawconfig.Paths.Lock, sessionId.String()+"-out-"+name)
	return flock.New(path, sessionId.String(), fcntllock.New)
}

func cacheDir() string {
	return filepath.Join(rawconfig.Paths.Cache, xsession.ID.String())
}
