package imon

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
)

func (t *Manager) getFrozen() time.Time {
	return file.ModTime(filepath.Join(t.path.VarDir(), "frozen"))
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
