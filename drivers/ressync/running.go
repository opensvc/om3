package ressync

import (
	"path/filepath"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/runfiles"
	"github.com/opensvc/om3/v3/util/xsession"
)

// RunDir holds a file per sync in progress, as the one of a task: the daemon
// watches it, and marks the resource and its instance running while a file is
// there.
func (t *T) RunDir() runfiles.Dir {
	return runfiles.Dir{
		Path: filepath.Join(t.VarDir(), "run"),
		Log:  t.Log(),
	}
}

// Running implements resource.Runninger: the syncs in progress, from their
// run files.
func (t *T) Running() (resource.RunningInfoList, error) {
	var l resource.RunningInfoList
	err := l.LoadRunDir(t.RID(), t.RunDir())
	return l, err
}

// StartRun records a sync in progress until the func it returns is called.
func (t *T) StartRun() (func(), error) {
	runDir := t.RunDir()
	if err := runDir.Create([]byte(xsession.SessionID().String())); err != nil {
		return func() {}, err
	}
	return func() {
		if err := runDir.Remove(); err != nil {
			t.Log().Warnf("remove run file: %s", err)
		}
	}, nil
}
