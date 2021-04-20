package flock

import (
	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/xsession"
	"path/filepath"
)

type (
	// T wraps flock and dumps a json data in the lock file
	// hinting about what holds the lock.
	// It get its lock from fcntllock
	T = flock.T
)

var (
	getPathVar = pathVar
)

// New allocate a file lock struct that use fnctllock.
func New(name string) *T {
	path := filepath.Join(getPathVar(), "lock", name)
	return flock.New(path, xsession.Id(), fcntllock.New)
}

func pathVar() string {
	return config.Node.Paths.Var
}
