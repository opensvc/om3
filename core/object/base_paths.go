package object

import (
	"os"

	log "github.com/sirupsen/logrus"
)

type BasePaths struct {
	varDir string
	logDir string
	tmpDir string
}

func (t *Base) varDir() string {
	if t.paths.varDir != "" {
		return t.paths.varDir
	}
	t.paths.varDir = t.Path.VarDir()
	if !t.Volatile {
		if err := os.MkdirAll(t.paths.varDir, os.ModePerm); err != nil {
			log.Error(err)
		}
	}
	return t.paths.varDir
}

func (t *Base) logDir() string {
	if t.paths.logDir != "" {
		return t.paths.logDir
	}
	t.paths.logDir = t.Path.LogDir()
	if !t.Volatile {
		if err := os.MkdirAll(t.paths.logDir, os.ModePerm); err != nil {
			log.Error(err)
		}
	}
	return t.paths.logDir
}

func (t *Base) tmpDir() string {
	if t.paths.tmpDir != "" {
		return t.paths.tmpDir
	}
	t.paths.tmpDir = t.Path.TmpDir()
	if !t.Volatile {
		if err := os.MkdirAll(t.paths.tmpDir, os.ModePerm); err != nil {
			log.Error(err)
		}
	}
	return t.paths.tmpDir
}
