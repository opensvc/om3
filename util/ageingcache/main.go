package ageingcache

import (
	"path/filepath"
	"time"

	"github.com/opensvc/fcache"
	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/xsession"
)

var maxLockDuration = 30 * time.Second

// Output manage output session function cache
func Output(o fcache.Outputter, sig string, maxAge time.Duration) (out []byte, err error) {
	cacheDir := cacheDir()
	age, err := fcache.Age(sig, cacheDir, maxLockDuration, outputLockP)
	if err == nil && age > maxAge {
		Clear(sig)
	}
	return fcache.Output(o, sig, cacheDir, maxLockDuration, outputLockP)
}

// Clear removes the current cached output
func Clear(sig string) error {
	return fcache.Clear(sig, cacheDir(), maxLockDuration, outputLockP)
}

func outputLockP(name string) fcache.Locker {
	sid := xsession.Sid().String()
	path := filepath.Join(rawconfig.Paths.Lock, "ageing-out-"+name)
	return flock.New(path, sid, fcntllock.New)
}

func cacheDir() string {
	return filepath.Join(rawconfig.Paths.Cache, "ageing")
}
