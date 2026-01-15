package imon

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/opensvc/om3/v3/util/file"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

type (
	ridFile struct {
		*resource.File
		rid string
	}

	ridFiles []ridFile

	filesManager struct {
		// states stores the resource files we store to avoid uneeded refetch
		states map[string]ridFile

		// attention stores a pending InstanceStatusUpdated event received while the fetch
		// manager was already processing an event. This serves as a flag to immediately
		// retrigger a new fetch cycle upon completion of the current one.
		attention *msgbus.InstanceStatusUpdated

		// fetching is true when the resource files fetch and ingest routine is
		// running
		fetching bool
	}
)

func newFilesManager() *filesManager {
	return &filesManager{
		states: make(map[string]ridFile),
	}
}

func (t ridFiles) Lookup(name string) (ridFile, bool) {
	for _, file := range t {
		if file.Name == name {
			return file, true
		}
	}
	return ridFile{}, false
}

func (t *filesManager) Fetched() ridFiles {
	l := make(ridFiles, len(t.states))
	i := 0
	for _, v := range t.states {
		l[i] = v
		i += 1
	}
	return l
}

func (t *Manager) handleResourceFiles(ev *msgbus.InstanceStatusUpdated) {
	if t.instConfig.ActorConfig == nil {
		return
	}
	if ev.Node == t.localhost {
		t.handleLocalResourceFiles(ev)
		return
	}
	if !t.needFetchResourceFiles(ev) {
		return
	}
	if t.files.fetching {
		t.files.attention = ev
		return
	}
	t.handlePeerResourceFiles(ev)
}

func (t *Manager) handlePeerResourceFiles(ev *msgbus.InstanceStatusUpdated) {
	t.files.attention = nil
	t.files.fetching = true
	go t.fetchResourceFiles(t.files.Fetched(), t.instStatus[t.localhost], ev)
}

func (t *Manager) needFetchResourceFiles(ev *msgbus.InstanceStatusUpdated) bool {
	// Is the local node ready to receive a resource file
	if t.state.LocalExpect != instance.MonitorLocalExpectNone {
		return false
	}

	// Is the remote node a valid resource file authority
	if ev.Value.Avail != status.Up {
		return false
	}

	return true
}

func (t *Manager) initLocalResourceFiles() {
	instanceStatus, ok := t.instStatus[t.localhost]
	if !ok {
		return
	}
	for rid, localResourceStatus := range instanceStatus.Resources {
		for _, f := range localResourceStatus.Files {
			t.log.Infof("%s: file %s discovered (csum=%s, mtime=%s)", rid, f.Name, f.Checksum, f.Mtime)
			t.files.states[f.Name] = ridFile{
				rid:  rid,
				File: &f,
			}
		}
	}
}

func (t *Manager) handleLocalResourceFiles(ev *msgbus.InstanceStatusUpdated) {
	var needFetch bool

	// Prepare a map of existing filenames (e.g. reported in the local
	// instance status).
	existingFilenames := make(map[string]any)
	for rid, localResourceStatus := range ev.Value.Resources {
		for _, f := range localResourceStatus.Files {
			existingFilenames[f.Name] = nil

			if ev.Value.Avail != status.Up {
				fetchedFile, ok := t.files.states[f.Name]
				if !ok {
					t.log.Infof("%s: file %s discovered (csum=%s, mtime=%s)", rid, f.Name, f.Checksum, f.Mtime)
					t.files.states[f.Name] = ridFile{
						rid:  rid,
						File: &f,
					}
					needFetch = true
				} else if fetchedFile.Checksum != f.Checksum {
					t.log.Infof("%s: file %s altered locally (csum=%s, mtime=%s)", rid, f.Name, f.Checksum, f.Mtime)
					t.files.states[f.Name] = ridFile{
						rid:  rid,
						File: &f,
					}
					needFetch = true
				}
			}
		}
	}

	// Prepare the list of filenames we fetched but are no longer existing
	var toDelete []string
	for filename, fetchedFile := range t.files.states {
		if _, ok := existingFilenames[filename]; !ok {
			toDelete = append(toDelete, filename)
			t.log.Infof("%s: file %s disappeared", fetchedFile.rid, filename)
			needFetch = true
		}
	}

	// Mark these filenames as not fetched
	for _, filename := range toDelete {
		delete(t.files.states, filename)
	}

	// If a file was changed or removed or discovered, fetch from the up instance
	if needFetch && ev.Value.Avail != status.Up {
		for nodename, instanceStatus := range instance.StatusData.GetByPath(t.path) {
			if t.localhost == nodename {
				continue
			}
			if instanceStatus.Avail != status.Up {
				continue
			}
			t.handleResourceFiles(&msgbus.InstanceStatusUpdated{
				Node:  nodename,
				Value: *instanceStatus,
			})
			break
		}
	}
}

func (t *Manager) fetchResourceFiles(fetched ridFiles, localInstanceStatus instance.Status, ev *msgbus.InstanceStatusUpdated) {

	var (
		ridsToIngest []string
		ridsNoIngest []string
	)

	done := cmdFetchDone{}

	for rid, peerResourceStatus := range ev.Value.Resources {
		if peerResourceStatus.Status != status.Up {
			continue
		}
		localResourceStatus, ok := localInstanceStatus.Resources[rid]
		if !ok {
			continue
		}
		if localResourceStatus.Status != status.Down {
			continue
		}
		for _, peerFile := range peerResourceStatus.Files {
			// try to get the localFile from the fetched cache first
			// because fetch cache contains more recent data than the
			// t.instStatus cache.
			localFile, ok := fetched.Lookup(peerFile.Name)
			if !ok {
				// fallback to the t.instStatus cache, ie we never fetched
				// this file yet.
				file, _ := localResourceStatus.Files.Lookup(peerFile.Name)
				localFile = ridFile{
					File: &file,
					rid:  rid,
				}
			}
			if peerFile.Checksum == localFile.Checksum {
				continue
			}
			if err := t.fetchResourceFile(rid, peerFile, ev.Node); err != nil {
				t.log.Warnf("%s: fetch %s: %s", rid, peerFile.Name, err)
				continue
			}

			done.Files = append(done.Files, ridFile{
				File: &peerFile,
				rid:  rid,
			})

			if peerFile.Ingest {
				ridsToIngest = append(ridsToIngest, rid)
			} else {
				ridsNoIngest = append(ridsNoIngest, rid)
			}
		}
	}
	if ridsToIngest != nil {
		if err := t.crmResourceIngest(ridsToIngest); err != nil {
			t.log.Warnf("%s", err)
		}
	} else if ridsNoIngest != nil {
		t.log.Tracef("no transfered resource file needs ingest")
	}
	t.cmdC <- done
}

func (t *Manager) fetchResourceFile(rid string, peerFile resource.File, from string) error {
	t.log.Infof("%s: file %s fetch from %s", rid, peerFile.Name, from)
	c, err := client.New(
		client.WithURL(daemonsubsystem.PeerURL(from)),
		client.WithUsername(t.localhost),
		client.WithPassword(cluster.ConfigData.Get().Secret()),
		client.WithCertificate(daemonenv.CertChainFile()),
	)
	if err != nil {
		return err
	}
	params := &api.GetInstanceResourceFileParams{
		Rid:  rid,
		Name: peerFile.Name,
	}
	resp, err := c.GetInstanceResourceFile(t.ctx, from, t.path.Namespace, t.path.Kind, t.path.Name, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected api response status %s", resp.Status)
	}

	// Create a temp file and write the response body
	tempFile, err := os.CreateTemp(filepath.Dir(peerFile.Name), filepath.Base(peerFile.Name)+".*")
	if err != nil {
		return err
	}
	defer tempFile.Close()

	tempFilename := tempFile.Name()
	defer os.Remove(tempFilename)

	if _, err = io.Copy(tempFile, resp.Body); err != nil {
		return err
	}

	b, err := file.MD5(tempFilename)
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", b) != peerFile.Checksum {
		return fmt.Errorf("the fetch resource file %s content has changed: don't install", peerFile.Name)
	}

	// Parse mtime, uid, gid, perm from the response headers
	tm, err := time.Parse(time.RFC3339Nano, resp.Header.Get(api.HeaderLastModified))
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(resp.Header.Get(api.HeaderUser))
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(resp.Header.Get(api.HeaderGroup))
	if err != nil {
		return err
	}
	perm, err := strconv.ParseUint(resp.Header.Get(api.HeaderPerm), 8, 32)
	if err != nil {
		return err
	}

	// Apply mtime, uid, gid, perm to the temp file
	if err := os.Chtimes(tempFilename, tm, tm); err != nil {
		return err
	}
	if os.Chown(tempFilename, uid, gid); err != nil {
		return err
	}
	if os.Chmod(tempFilename, os.FileMode(perm)); err != nil {
		return err
	}

	// Atomic file replace
	if err := os.Rename(tempFilename, peerFile.Name); err != nil {
		return err
	}
	return nil
}

func (t *Manager) onFetchDone(c cmdFetchDone) {
	// Remember we fetched this file to avoid re-fetch if we recv
	// another peer InstanceStatusUpdated before we update our
	// own InstanceStatusUpdated
	for _, file := range c.Files {
		t.files.states[file.Name] = file
	}
	t.files.fetching = false
	if t.files.attention != nil {
		t.handlePeerResourceFiles(t.files.attention)
	}
}
