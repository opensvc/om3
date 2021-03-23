package object

import (
	"opensvc.com/opensvc/config"
)

func (t *Base) loadConfig() error {
	var err error
	t.config, err = config.NewObject(t.Path.ConfigFile())
	return err
}

func (t Base) Config() *config.T {
	return t.config
}
