package config

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type (
	Type struct {
		Path string
		v    *viper.Viper
		raw  map[string]interface{}
	}
)

func (t *Type) Get(key string) interface{} {
	val := t.v.Get(key)
	log.Debugf("config %s get %s => %s", t.Path, key, val)
	return val
}
