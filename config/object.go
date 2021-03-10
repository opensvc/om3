package config

import (
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// LoadObject configures and returns a Viper instance
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

	log.Debugf("new config for %s: %d sections", p, len(t.raw))
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
