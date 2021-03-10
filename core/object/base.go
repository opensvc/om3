package object

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"opensvc.com/opensvc/config"
)

type (
	// Base is the base struct embedded in all kinded objects.
	Base struct {
		Path   Path
		config *config.Type
	}
)

// Status returns the service status dataset
func (o *Base) Status(refresh bool) error {
	return nil
}

// List returns the stringified path as data
func (o *Base) List() (string, error) {
	return o.Path.String(), nil
}

// Start starts the local instance of the object
func (o *Base) Start() error {
	return nil
}

// Get gets keyword values
func (o *Base) Get(kws []string) ([]string, error) {
	data := make([]string, 0)
	for _, kw := range kws {
		data = append(data, o.config.Get(kw).(string))
	}
	return data, nil
}

func (o *Base) init(path Path) error {
	o.Path = path
	if err := o.loadConfig(); err != nil {
		log.Debugf("%s init error: %s", o, err)
		return err
	}
	log.Debugf("%s initialized", o)
	return nil
}

func (o Base) String() string {
	return fmt.Sprintf("base object %s", o.Path)
}

func (o *Base) loadConfig() error {
	var err error
	o.config, err = config.NewObject(o.Path.ConfigFile())
	return err
}
