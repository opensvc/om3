package config

import (
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/spf13/viper"
)

// NewObject configures and returns a Viper instance
func NewObject(p string) (*Type, error) {
	t := &Type{
		Path: p,
	}
	t.v = viper.New()
	t.v.SetConfigType("ini")
	t.v.SetConfigFile(filepath.FromSlash(p))
	t.v.ReadInConfig()
	t.raw = make(map[string]interface{})

	if err := t.v.Unmarshal(&t.raw); err != nil {
		return nil, err
	}

	log.Debug().Msgf("new config for %s: %d sections", p, len(t.raw))
	return t, nil
}

/*
	defaults, ok := data["default"]
	if !ok {
		data["defaults"] = map[string]string{
			"nodes": Node.Hostname,
		}
	}
*/
