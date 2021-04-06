package config

import (
	"path/filepath"

	"github.com/rs/zerolog/log"
	"gopkg.in/ini.v1"

	"github.com/spf13/viper"
)

// NewObject configures and returns a Viper instance
func NewObject(p string) (*T, error) {
	cf := filepath.FromSlash(p)
	t := &T{
		ConfigFilePath: cf,
	}
	t.v = viper.NewWithOptions(viper.IniLoadOptions(ini.LoadOptions{
		Loose:                      true,
		AllowPythonMultilineValues: true,
		SpaceBeforeInlineComment:   true,
	}))
	t.v.SetConfigType("ini")
	t.v.SetConfigFile(cf)
	t.v.AddConfigPath(filepath.Dir(cf))
	t.v.SetConfigName(filepath.Base(cf))
	t.v.ReadInConfig()

	t.raw = make(Raw)

	if err := t.v.Unmarshal(&t.raw); err != nil {
		return nil, err
	}

	log.Debug().Msgf("new config for %s: %d sections", p, len(t.raw))
	return t, nil
}
