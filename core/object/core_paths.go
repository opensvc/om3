package object

import (
	"os"
)

func (t *core) VarDir() string {
	return t.path.VarDir()
}

func (t *core) varDir() string {
	if t.paths.varDir != "" {
		return t.paths.varDir
	}
	t.paths.varDir = t.path.VarDir()
	if !t.volatile {
		if err := os.MkdirAll(t.paths.varDir, os.ModePerm); err != nil {
			t.log.Error().Msgf("%s", err)
		}
	}
	return t.paths.varDir
}

func (t *core) LogDir() string {
	return t.path.LogDir()
}

func (t *core) logDir() string {
	if t.paths.logDir != "" {
		return t.paths.logDir
	}
	t.paths.logDir = t.path.LogDir()
	if !t.volatile {
		if err := os.MkdirAll(t.paths.logDir, os.ModePerm); err != nil {
			t.log.Error().Msgf("%s", err)
		}
	}
	return t.paths.logDir
}

func (t *core) TmpDir() string {
	return t.path.TmpDir()
}

func (t *core) tmpDir() string {
	if t.paths.tmpDir != "" {
		return t.paths.tmpDir
	}
	t.paths.tmpDir = t.path.TmpDir()
	if !t.volatile {
		if err := os.MkdirAll(t.paths.tmpDir, os.ModePerm); err != nil {
			t.log.Error().Msgf("%s", err)
		}
	}
	return t.paths.tmpDir
}
