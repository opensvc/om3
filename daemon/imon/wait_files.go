package imon

import (
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// waitFilesTimeout bounds the wait of the destination of a placement for
// the resource files of the source it takes over from. The files are on the
// disk of the source, a fetch away: a wait longer than that is a fetch that
// does not succeed.
const waitFilesTimeout = time.Minute

// pendingSourceFiles returns the stopped sources of the placement under way
// (see isStoppedSource), and the files they list that the local instance
// does not hold in the version listed, both sorted.
//
// Only the files a fetch takes count: the ones of the resources down here,
// of the same scope on both nodes, in a directory this node has (see
// fetchResourceFiles). The version is the checksum the source reports, and
// not a modification time, which the clocks of two nodes would not compare.
func (t *Manager) pendingSourceFiles() ([]string, []string) {
	sources := make([]string, 0)
	files := make([]string, 0)
	if t.files == nil {
		return sources, files
	}
	local := t.instStatus[t.localhost]
	for _, nodename := range t.stoppedSources() {
		n := 0
		for rid, peerResourceStatus := range t.instStatus[nodename].Resources {
			localResourceStatus, ok := local.Resources[rid]
			if !ok || localResourceStatus.Status != status.Down {
				continue
			}
			if !sameSyncScope(peerResourceStatus, localResourceStatus) {
				continue
			}
			for _, peerFile := range peerResourceStatus.Files {
				if t.holdsResourceFile(localResourceStatus, peerFile) {
					continue
				}
				if _, ok := fetchDir(peerFile.Name); !ok {
					continue
				}
				files = append(files, peerFile.Name)
				n++
			}
		}
		if n > 0 {
			sources = append(sources, nodename)
		}
	}
	sort.Strings(files)
	return sources, files
}

// holdsResourceFile says whether the local instance holds a file in the
// version a peer lists: the version fetched last, or else the one the local
// status lists, as fetchResourceFiles reads them.
func (t *Manager) holdsResourceFile(localResourceStatus resource.Status, peerFile resource.File) bool {
	if fetched, ok := t.files.states[peerFile.Name]; ok {
		return fetched.Checksum == peerFile.Checksum
	}
	localFile, _ := localResourceStatus.Files.Lookup(peerFile.Name)
	return localFile.Checksum == peerFile.Checksum
}

// setWaitFiles holds the start of the destination of a placement until it
// holds the resource files the source lists, and reports whether it does.
//
// A file changed shortly before a move, as the definition of a kvm domain
// the object was just given, is listed first by the status the source
// refreshes at the end of its stop. No instance is up by then to fetch it
// from, and the destination started without it: "definition not found",
// start failed. The destination fetches the files from the source it takes
// over from, the authority while the orchestration lasts, and starts once
// the fetch is done.
//
// The wait lasts waitFilesTimeout at most. Past it, the start fails, naming
// the files and the source, rather than running without them.
func (t *Manager) setWaitFiles() bool {
	sources, files := t.pendingSourceFiles()
	waiting := t.state.State == instance.MonitorStateWaitFiles && !t.waitFilesSince.IsZero()
	if len(files) == 0 {
		if waiting {
			t.log.Infof("resource files fetched, start")
			t.waitFilesSince = time.Time{}
			t.waitFilesTimer.Stop()
		}
		return false
	}
	if !waiting {
		t.log.Infof("wait for the resource files %s of %s before starting, %s at most",
			strings.Join(files, ","), strings.Join(sources, ","), waitFilesTimeout)
		t.waitFilesSince = time.Now()
		t.waitFilesTimer.Reset(waitFilesTimeout)
		t.transitionTo(instance.MonitorStateWaitFiles)
		t.fetchSourceFiles(sources)
		return true
	}
	if time.Since(t.waitFilesSince) > waitFilesTimeout {
		t.failWaitFiles(sources, files)
	}
	return true
}

// fetchSourceFiles fetches the files of the sources, as their status would
// have had it done on its arrival, the fetch of a status that arrived before
// the source was known stopped by the orchestration included. A fetch
// running already is followed by this one.
func (t *Manager) fetchSourceFiles(sources []string) {
	for _, nodename := range sources {
		t.handleResourceFiles(&msgbus.InstanceStatusUpdated{
			Path:  t.path,
			Node:  nodename,
			Value: t.instStatus[nodename],
		})
	}
}

// failWaitFiles ends the placement the destination waited for the files
// of, the instance not started: the start failed, terminal as any.
func (t *Manager) failWaitFiles(sources, files []string) {
	t.loggerWithState().Errorf("orchestration %s: resource files %s of %s not fetched after %s: the instance is not started",
		t.state.GlobalExpect, strings.Join(files, ","), strings.Join(sources, ","), waitFilesTimeout)
	t.waitFilesSince = time.Time{}
	t.waitFilesTimer.Stop()
	t.transitionTo(instance.MonitorStateStartFailure)
	t.done()
	t.clearPending()
}

// leaveWaitFiles puts back to idle an instance an aborted orchestration
// leaves waiting for files.
func (t *Manager) leaveWaitFiles() {
	if t.state.State != instance.MonitorStateWaitFiles {
		return
	}
	t.log.Infof("orchestration aborted: stop waiting for the resource files")
	t.waitFilesSince = time.Time{}
	t.waitFilesTimer.Stop()
	t.change = true
	t.state.State = instance.MonitorStateIdle
}

// onWaitFilesTimer has the orchestration look again at the files it waits
// for, its time being up, when no fetch did.
func (t *Manager) onWaitFilesTimer() {
	if t.state.State != instance.MonitorStateWaitFiles {
		return
	}
	t.onChange()
}
