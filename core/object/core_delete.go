package object

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/util/file"
)

//
// Delete is the 'delete' object action entrypoint.
//
// If no resource selector is set, remove all etc, var and log
// file belonging to the object.
//
// If a resource selector is set, only delete the corresponding
// sections in the configuration file.
//
func (t core) DeleteSection(ctx context.Context, rid string) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Delete)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.deleteSections(rid)
}

func (t core) Delete(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Delete)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.deleteInstance()
}

func (t core) deleteInstance() error {
	if err := t.deleteInstanceFiles(); err != nil {
		return err
	}
	if err := t.deleteInstanceLogs(); err != nil {
		return err
	}
	if err := t.setPurgeCollectorTag(); err != nil {
		t.log.Warn().Err(err).Msg("")
		return nil
	}
	return nil
}

func (t core) deleteInstanceFiles() error {
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
			t.log.Warn().Err(err).Str("pattern", pattern).Msg("expand glob for delete")
			continue
		}
		for _, fpath := range matches {
			_ = t.tryDeleteInstanceFile(fpath)
		}
	}
	t.tryDeleteInstanceFile(t.ConfigFile())
	return nil
}

func (t core) tryDeleteInstanceFile(fpath string) bool {
	if file.IsProtected(fpath) {
		t.log.Warn().Str("path", fpath).Msg("block attempt to remove a protected file")
		return false
	}
	if err := os.RemoveAll(fpath); err != nil {
		t.log.Warn().Err(err).Str("path", fpath).Msg("removing")
		return false
	}
	t.log.Info().Str("path", fpath).Msg("removed")
	return true
}

func (t core) deleteInstanceLogs() error {
	return nil
}

func (t core) setPurgeCollectorTag() error {
	return nil
}

func (t core) deleteSections(s string) error {
	sections := strings.Split(s, ",")
	return t.config.DeleteSections(sections)
}
