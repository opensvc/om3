package xconfig

import (
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/ini.v1"
)

// NewObject configures and returns a Viper instance
func NewObject(p string, others ...interface{}) (t *T, err error) {
	cf := filepath.FromSlash(p)
	t = &T{
		ConfigFilePath: cf,
	}
	t.file, err = ini.LoadSources(ini.LoadOptions{
		Loose:                      true,
		AllowPythonMultilineValues: true,
		SpaceBeforeInlineComment:   true,
	}, cf, others...)
	if err != nil {
		return nil, errors.Wrap(err, "load config error")
	}
	return t, nil
}
