package imon

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/flagfile"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/file"
)

func (t *Manager) getFrozen() time.Time {
	return file.ModTime(t.path.FrozenFile())
}

// setStopped raises the flag saying the instance was stopped on purpose, so
// the daemon does not start it back on its own, and publishes
// InstanceStoppedFileUpdated. The local instance status cache is updated with
// the value read from the file system.
//
// A stop the operator asked for raises it. A stop the daemon decided on by
// itself, like the one that moves an object to another node, does not: the
// object is still wanted up.
func (t *Manager) setStopped() error {
	p := t.path.StoppedFile()
	if err := flagfile.Set(p); err != nil {
		t.log.Errorf("set the stopped flag: %s", err)
		return err
	}
	stopped := flagfile.At(p)
	if stopped.IsZero() {
		err := fmt.Errorf("unexpected stopped flag reset on %s", p)
		t.log.Errorf("set the stopped flag: %s", err)
		return err
	}
	if instanceStatus, ok := t.instStatus[t.localhost]; ok {
		instanceStatus.StoppedAt = stopped
		t.instStatus[t.localhost] = instanceStatus
	}
	t.publisher.Pub(&msgbus.InstanceStoppedFileUpdated{Path: t.path, File: p, At: stopped}, t.pubLabels...)
	return nil
}

// unsetStopped lowers the flag saying the instance was stopped on purpose, so
// the daemon may start it on its own again, and publishes
// InstanceStoppedFileRemoved.
func (t *Manager) unsetStopped() error {
	p := t.path.StoppedFile()
	if flagfile.At(p).IsZero() {
		return nil
	}
	if err := flagfile.Unset(p); err != nil {
		t.log.Errorf("unset the stopped flag: %s", err)
		return err
	}
	if instanceStatus, ok := t.instStatus[t.localhost]; ok {
		instanceStatus.StoppedAt = time.Time{}
		t.instStatus[t.localhost] = instanceStatus
	}
	t.publisher.Pub(&msgbus.InstanceStoppedFileRemoved{Path: t.path, File: p, At: time.Now()}, t.pubLabels...)
	return nil
}

// isStopped says the local instance was stopped on purpose.
func (t *Manager) isStopped() bool {
	return t.instStatus[t.localhost].IsStopped()
}

// freeze creates missing instance frozen flag file, and publish InstanceFrozenFileUpdated
// local instance status cache frozen value is updated with value read from file system
func (t *Manager) freeze() error {
	frozen := t.getFrozen()

	t.log.Tracef("daemon action freeze")
	p := filepath.Join(t.path.VarDir(), "frozen")

	if !file.Exists(p) {
		d := filepath.Dir(p)
		if !file.Exists(d) {
			if err := os.MkdirAll(d, os.ModePerm); err != nil {
				t.log.Errorf("freeze: %s", err)
				return err
			}
		}
		f, err := os.Create(p)
		if err != nil {
			t.log.Errorf("freeze: %s", err)
			return err
		}
		_ = f.Close()
	}
	frozen = file.ModTime(p)
	if instanceStatus, ok := t.instStatus[t.localhost]; ok {
		instanceStatus.FrozenAt = frozen
		t.instStatus[t.localhost] = instanceStatus
	}
	if frozen.IsZero() {
		err := fmt.Errorf("unexpected frozen reset on %s", p)
		t.log.Errorf("freeze: %s", err)
		return err
	}
	t.publisher.Pub(&msgbus.InstanceFrozenFileUpdated{Path: t.path, At: frozen}, t.pubLabels...)
	return nil
}

// freeze removes instance frozen flag file, and publish InstanceFrozenFileUpdated
// local instance status cache frozen value is updated with value read from file system
func (t *Manager) unfreeze() error {
	t.log.Tracef("daemon action unfreeze")
	p := filepath.Join(t.path.VarDir(), "frozen")
	if !file.Exists(p) {
		t.log.Infof("already unfrozen")
	} else {
		err := os.Remove(p)
		if err != nil {
			t.log.Errorf("unfreeze: %s", err)
			return err
		}
	}
	if instanceStatus, ok := t.instStatus[t.localhost]; ok {
		instanceStatus.FrozenAt = time.Time{}
		t.instStatus[t.localhost] = instanceStatus
	}
	t.publisher.Pub(&msgbus.InstanceFrozenFileRemoved{Path: t.path, At: time.Now()}, t.pubLabels...)
	return nil
}
