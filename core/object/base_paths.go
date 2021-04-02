package object

import (
	"os"
)

// BasePaths contains lazy initialized object paths on the node filesystem.
type BasePaths struct {
	varDir string
	logDir string
	tmpDir string
}

func (t *Base) varDir() string {
	if t.paths.varDir != "" {
		return t.paths.varDir
	}
	t.paths.varDir = t.VarDir()
	if !t.Volatile {
		if err := os.MkdirAll(t.paths.varDir, os.ModePerm); err != nil {
			t.log.Error().Msgf("%s", err)
		}
	}
	return t.paths.varDir
}

func (t *Base) logDir() string {
	if t.paths.logDir != "" {
		return t.paths.logDir
	}
	t.paths.logDir = t.LogDir()
	if !t.Volatile {
		if err := os.MkdirAll(t.paths.logDir, os.ModePerm); err != nil {
			t.log.Error().Msgf("%s", err)
		}
	}
	return t.paths.logDir
}

func (t *Base) tmpDir() string {
	if t.paths.tmpDir != "" {
		return t.paths.tmpDir
	}
	t.paths.tmpDir = t.TmpDir()
	if !t.Volatile {
		if err := os.MkdirAll(t.paths.tmpDir, os.ModePerm); err != nil {
			t.log.Error().Msgf("%s", err)
		}
	}
	return t.paths.tmpDir
}
