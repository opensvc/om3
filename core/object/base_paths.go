package object

import (
	"fmt"
	"os"
	"path/filepath"

	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
)

// BasePaths contains lazy initialized object paths on the node filesystem.
type BasePaths struct {
	varDir string
	logDir string
	tmpDir string
}

//
// VarDir returns the directory on the local filesystem where the object
// variable persistent data is stored as files.
//
func VarDir(p path.T) string {
	var s string
	switch p.Namespace {
	case "", "root":
		s = fmt.Sprintf("%s/%s/%s", rawconfig.Paths.Var, p.Kind, p.Name)
	default:
		s = fmt.Sprintf("%s/namespaces/%s", rawconfig.Paths.Var, p)
	}
	return filepath.FromSlash(s)
}

//
// TmpDir returns the directory on the local filesystem where the object
// stores its temporary files.
//
func TmpDir(p path.T) string {
	var s string
	switch {
	case p.Namespace != "", p.Namespace != "root":
		s = fmt.Sprintf("%s/namespaces/%s/%s", rawconfig.Paths.Tmp, p.Namespace, p.Kind)
	case p.Kind == kind.Svc, p.Kind == kind.Ccfg:
		s = fmt.Sprintf("%s", rawconfig.Paths.Tmp)
	default:
		s = fmt.Sprintf("%s/%s", rawconfig.Paths.Tmp, p.Kind)
	}
	return filepath.FromSlash(s)
}

//
// LogDir returns the directory on the local filesystem where the object
// stores its temporary files.
//
func LogDir(p path.T) string {
	var s string
	switch {
	case p.Namespace != "", p.Namespace != "root":
		s = fmt.Sprintf("%s/namespaces/%s/%s", rawconfig.Paths.Log, p.Namespace, p.Kind)
	case p.Kind == kind.Svc, p.Kind == kind.Ccfg:
		s = fmt.Sprintf("%s", rawconfig.Paths.Log)
	default:
		s = fmt.Sprintf("%s/%s", rawconfig.Paths.Log, p.Kind)
	}
	return filepath.FromSlash(s)
}

func LogFile(p path.T) string {
	return filepath.Join(LogDir(p), p.Name+".log")
}

func (t *Base) VarDir() string {
	return VarDir(t.Path)
}

func (t *Base) varDir() string {
	if t.paths.varDir != "" {
		return t.paths.varDir
	}
	t.paths.varDir = VarDir(t.Path)
	if !t.volatile {
		if err := os.MkdirAll(t.paths.varDir, os.ModePerm); err != nil {
			t.log.Error().Msgf("%s", err)
		}
	}
	return t.paths.varDir
}

func (t *Base) LogDir() string {
	return LogDir(t.Path)
}

func (t *Base) logDir() string {
	if t.paths.logDir != "" {
		return t.paths.logDir
	}
	t.paths.logDir = LogDir(t.Path)
	if !t.volatile {
		if err := os.MkdirAll(t.paths.logDir, os.ModePerm); err != nil {
			t.log.Error().Msgf("%s", err)
		}
	}
	return t.paths.logDir
}

func (t *Base) TmpDir() string {
	return TmpDir(t.Path)
}

func (t *Base) tmpDir() string {
	if t.paths.tmpDir != "" {
		return t.paths.tmpDir
	}
	t.paths.tmpDir = TmpDir(t.Path)
	if !t.volatile {
		if err := os.MkdirAll(t.paths.tmpDir, os.ModePerm); err != nil {
			t.log.Error().Msgf("%s", err)
		}
	}
	return t.paths.tmpDir
}
