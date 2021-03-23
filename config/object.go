package config

import (
	"path/filepath"

	"github.com/rs/zerolog/log"
	"gopkg.in/ini.v1"

	"github.com/spf13/viper"
)

// NewObject configures and returns a Viper instance
func NewObject(p string) (*T, error) {
	t := &T{
		Path: p,
	}
	t.v = viper.NewWithOptions(viper.IniLoadOptions(ini.LoadOptions{
		Loose:                      true,
		AllowPythonMultilineValues: true,
		SpaceBeforeInlineComment:   true,
	}))
	t.v.SetConfigType("ini")
	t.v.SetConfigFile(filepath.FromSlash(p))
	t.v.ReadInConfig()

	t.raw = make(Raw)

	if err := t.v.Unmarshal(&t.raw); err != nil {
		return nil, err
	}

	log.Debug().Msgf("new config for %s: %d sections", p, len(t.raw))
	return t, nil
}
