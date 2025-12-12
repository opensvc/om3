package object

import (
	"context"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/util/file"
)

// DeleteSection deletes a resource section from object.
//
// TODO: Fix/verify following doc
// If no resource selector is set, remove all etc, var and log
// file belonging to the object.
//
// If a resource selector is set, only delete the corresponding
// sections in the configuration file.
func (t *core) DeleteSection(ctx context.Context, rids ...string) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Delete)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.config.DeleteSections(rids...)
}

func (t *core) Delete(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Delete)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.deleteInstance()
}

func (t *core) deleteInstance() error {
	if err := t.deleteInstanceFiles(); err != nil {
		return err
	}
	if err := t.deleteInstanceLogs(); err != nil {
		return err
	}
	if err := t.setPurgeCollectorTag(); err != nil {
		t.log.Warnf("%s", err)
		return nil
	}
	return nil
}

func (t *core) deleteInstanceFiles() error {
	patterns := []string{
		filepath.Join(t.logDir(), t.path.Name+".log*"),
		filepath.Join(t.logDir(), t.path.Name+".debug.log*"),
		filepath.Join(t.logDir(), "."+t.path.Name+".log*"),
		filepath.Join(t.logDir(), "."+t.path.Name+".debug.log*"),
		filepath.Join(t.varDir()),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.log.Attr("pattern", pattern).Warnf("expand glob for delete: %s", err)
			continue
		}
		for _, fpath := range matches {
			_ = t.tryDeleteInstanceFile(fpath)
		}
	}
	t.tryDeleteInstanceFile(t.ConfigFile())
	return nil
}

func (t *core) tryDeleteInstanceFile(fpath string) bool {
	if file.IsProtected(fpath) {
		t.log.Attr("path", fpath).Warnf("block attempt to remove the protected file %s", fpath)
		return false
	}
	if err := os.RemoveAll(fpath); err != nil {
		t.log.Attr("path", fpath).Warnf("removing %s: %s", fpath, err)
		return false
	}
	t.log.Attr("path", fpath).Infof("removed %s", fpath)
	return true
}

func (t *core) deleteInstanceLogs() error {
	return nil
}

func (t *core) setPurgeCollectorTag() error {
	return nil
}
