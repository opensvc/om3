package config

import (
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// LoadObject configures and returns a Viper instance
func LoadObject(p string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigType("ini")
	v.SetConfigFile(filepath.FromSlash(p))
	v.ReadInConfig()

	data := make(map[string]interface{})

	if err := v.Unmarshal(&data); err != nil {
		return nil, err
	}
	defaults, ok := data["DEFAULT"]
	if !ok {
		defaults = map[string]string{
			"nodes": Node.Hostname,
		}
	}
	log.Debugf("config loaded from %s: %s", p, defaults)
	return v, nil
}
