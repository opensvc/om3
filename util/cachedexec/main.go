// Package cachedexec provides primitives to get cmd results from a cache
package cachedexec

import (
	"io/ioutil"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/flock"
	"os"
	"os/exec"
	"strings"
	"time"
)

type (
	T struct {
		*exec.Cmd
		sessionId string
		lockFile  string
		dataFile  string
	}

	locker interface {
		Lock(time.Duration, string) (err error)
		UnLock() (err error)
	}

//locker = flock.Locker
)

var (
	lockProvider = flock.New
)

func New(cmd *exec.Cmd, s ...string) *T {
	path := strings.Join(s, "_") + strings.ReplaceAll(cmd.Path, "/", "_")
	return &T{
		Cmd:       cmd,
		sessionId: "",
		lockFile:  path + ".lock",
		dataFile:  path,
	}
}

// Output return exec.Cmd cached result
func (t T) Output() (out []byte, err error) {
	var lock locker
	lock = lockProvider(t.lockFile, t.sessionId)
	if err = lock.Lock(30*time.Second, "cache"); err != nil {
		return t.Cmd.Output()
	}
	defer func(lock locker) {
		_ = lock.UnLock()
	}(lock)
	if t.dataExist() {
		return ioutil.ReadFile(t.dataFile)
	}
	out, err = t.Cmd.Output()
	if err != nil {
		return nil, err
	}
	err = ioutil.WriteFile(t.dataFile, out, 0600)
	return out, err
}

// Clear remove exec.Cmd cached result
func (t T) Clear() (err error) {
	var lock locker
	lock = flock.New(t.lockFile, t.sessionId)
	if err = lock.Lock(30*time.Second, "cache"); err != nil {
		return
	}
	defer func(lock locker) {
		_ = lock.UnLock()
	}(lock)
	if t.dataExist() {
		return os.Remove(t.dataFile)
	}
	return
}

func (t T) dataExist() bool {
	return file.Exists(t.dataFile)
}

func (t T) cachePath(s []string) (path string) {
	path = strings.Join(s, "_") + strings.ReplaceAll(t.Path, "/", "(slash)")
	return
}
