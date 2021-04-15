package config

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/ini.v1"
)

// NewObject configures and returns a Viper instance
func NewObject(p string) (t *T, err error) {
	cf := filepath.FromSlash(p)
	t = &T{
		ConfigFilePath: cf,
	}
	t.file, err = ini.LoadSources(ini.LoadOptions{
		Loose:                      true,
		AllowPythonMultilineValues: true,
		SpaceBeforeInlineComment:   true,
	}, cf)
	if err != nil {
		return nil, errors.Wrap(err, "load config error")
	}
	log.Debug().Msgf("new config for %s: %d sections", p, len(t.file.Sections()))
	return t, nil
}
