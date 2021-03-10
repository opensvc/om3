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

	// ActionResult is a predictible type of actions return value, for reflect
	ActionResult struct {
		Path  Path
		Error error
		Data  interface{}
		Panic interface{}
	}
)

// NewActionResult allocate a new object action result, setting the path
// automatically.
func (o *Base) NewActionResult() *ActionResult {
	return &ActionResult{
		Path: o.Path,
	}
}

// Status returns the service status dataset
func (o *Base) Status(refresh bool) ActionResult {
	return *o.NewActionResult()
}

// List returns the stringified path as data
func (o *Base) List() ActionResult {
	result := o.NewActionResult()
	result.Data = o.Path.String()
	return *result
}

// Start starts the local instance of the object
func (o *Base) Start() ActionResult {
	result := o.NewActionResult()
	_ = o.config.Get("default.nodes")
	return *result
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
