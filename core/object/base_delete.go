package object

import (
	"os"
	"path/filepath"
	"strings"
)

type OptsDelete struct {
	Global           OptsGlobal
	Lock             OptsLocking
	ResourceSelector string `flag:"rid"`
	Unprovision      bool   `flag:"unprovision"`
}

//
// Delete is the 'delete' object action entrypoint.
//
// If no resource selector is set, remove all etc, var and log
// file belonging to the object.
//
// If a resource selector is set, only delete the corresponding
// sections in the configuration file.
//
func (t Base) Delete(opts OptsDelete) error {
	if opts.ResourceSelector != "" {
		return t.deleteSections(opts.ResourceSelector)
	}
	return t.deleteInstance()
}

func (t Base) deleteInstance() error {
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

func (t Base) deleteInstanceFiles() error {
	patterns := []string{
		filepath.Join(t.logDir(), t.Path.Name+".log*"),
		filepath.Join(t.logDir(), t.Path.Name+".debug.log*"),
		filepath.Join(t.logDir(), "."+t.Path.Name+".log*"),
		filepath.Join(t.logDir(), "."+t.Path.Name+".debug.log*"),
		filepath.Join(t.varDir(), "frozen"),
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

func (t Base) tryDeleteInstanceFile(fpath string) bool {
	if err := os.RemoveAll(fpath); err != nil {
		t.log.Warn().Err(err).Str("path", fpath).Msg("removing")
		return false
	}
	t.log.Info().Str("path", fpath).Msg("removed")
	return true
}

func (t Base) deleteInstanceLogs() error {
	return nil
}

func (t Base) setPurgeCollectorTag() error {
	return nil
}

func (t Base) deleteSections(ResourceSelector string) error {
	sections := strings.Split(ResourceSelector, ",")
	return t.config.DeleteSections(sections)
}
