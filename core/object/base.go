package object

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"opensvc.com/opensvc/config"
)

type (
	// Base is the base struct embedded in all kinded objects.
	Base struct {
		Path     Path
		Volatile bool

		// caches
		config *config.Type
		paths  BasePaths
	}
)

// Status returns the service status dataset
func (t *Base) Status(refresh bool) error {
	return nil
}

// List returns the stringified path as data
func (t *Base) List() (string, error) {
	return t.Path.String(), nil
}

// Start starts the local instance of the object
func (t *Base) Start(options ActionOptionsStart) error {
	lock, err := t.Lock("", options.LockTimeout, "start")
	if err != nil {
		return err
	}
	defer lock.Unlock()
	time.Sleep(10 * time.Second)
	return nil
}

// Get gets a keyword value
func (t *Base) Get(kw string) (string, error) {
	return t.config.Get(kw).(string), nil
}

func (t *Base) init(path Path) error {
	t.Path = path
	if err := t.loadConfig(); err != nil {
		log.Debugf("%s init error: %s", t, err)
		return err
	}
	log.Debugf("%s initialized", t)
	return nil
}

func (t Base) String() string {
	return fmt.Sprintf("base object %s", t.Path)
}

func (t *Base) loadConfig() error {
	var err error
	t.config, err = config.NewObject(t.Path.ConfigFile())
	return err
}
