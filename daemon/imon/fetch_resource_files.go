package imon

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/opensvc/om3/util/file"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/msgbus"
)

func (t *Manager) manageResourceFiles(ev *msgbus.InstanceStatusUpdated) {
	if !t.needFetchResourceFiles(ev) {
		return
	}
	t.fetchResourceFiles(ev)
}

func (t *Manager) needFetchResourceFiles(ev *msgbus.InstanceStatusUpdated) bool {
	// Is the local node ready to receive a resource file
	if t.state.LocalExpect != instance.MonitorLocalExpectNone {
		return false
	}
	if t.instConfig.ActorConfig == nil {
		return false
	}

	// Is the remote node a valid resource file authority
	if ev.Node == t.localhost {
		return false
	}
	if ev.Value.Avail != status.Up {
		return false
	}

	return true
}

func (t *Manager) fetchResourceFiles(ev *msgbus.InstanceStatusUpdated) {
	localInstanceStatus := t.instStatus[t.localhost]

	var (
		ridsToIngest []string
		ridsNoIngest []string
	)

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
			localFile, ok := localResourceStatus.Files.Lookup(peerFile.Name)
			if !ok {
				continue
			}
			if !peerFile.Mtime.After(localFile.Mtime) {
				continue
			}
			if peerFile.Checksum == localFile.Checksum {
				continue
			}
			if err := t.fetchResourceFile(rid, peerFile, ev.Node); err != nil {
				t.log.Warnf("%s", err)
				continue
			}

			// Remember we fetched this file to avoid re-fetch if we recv
			// another peer InstanceStatusUpdated before we update our
			// own InstanceStatusUpdated
			r := t.instStatus[t.localhost].Resources[rid]
			r.Files = r.Files.Merge(peerFile)
			t.instStatus[t.localhost].Resources[rid] = r

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
		t.log.Debugf("no transfered resource file needs ingest")
	}
}

func (t *Manager) fetchResourceFile(rid string, peerFile resource.File, from string) error {
	t.log.Infof("%s: fetch %s from %s", rid, peerFile.Name, from)
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
		return fmt.Errorf("%s: fetch %s: %s", rid, peerFile.Name, resp.Status)
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
