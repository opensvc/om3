package object

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

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
		varDir string
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
	if err := o.Lock("", 10); err != nil {
		return err
	}
	defer o.Unlock("")
	time.Sleep(10 * time.Second)
	return nil
}

// Get gets a keyword value
func (o *Base) Get(kw string) (string, error) {
	return o.config.Get(kw).(string), nil
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

// VarDir is the directory hosting the object's variable files
func (o *Base) VarDir() string {
	if o.varDir != "" {
		return o.varDir
	}
	o.varDir = o.Path.VarDir()
	if !o.Volatile {
		if err := os.MkdirAll(o.varDir, os.ModePerm); err != nil {
			log.Error(err)
		}
	}
	return o.varDir
}

// LockFile is the path of the file to use as an action lock.
func (o *Base) LockFile(group string) string {
	p := filepath.Join(o.VarDir(), "lock.generic")
	if group != "" {
		p += "." + group
	}
	return p
}

//
// Lock acquires the action lock.
//
// A custom lock group can be specified to prevent parallel run of a subset
// of object actions.
//
func (o *Base) Lock(group string, timeout time.Duration) error {
	p := o.LockFile(group)
	log.Debugf("locking %s", p)
	f := flock.New(p)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := f.TryLockContext(ctx, 1*time.Second)
	if err != nil {
		return err
	}
	log.Debugf("locked %s", p)
	return nil
}

//
// Unlock releases the action lock.
//
// A custom lock group can be specified to prevent parallel run of a subset
// of object actions.
//
func (o *Base) Unlock(group string) error {
	p := o.LockFile(group)
	log.Debugf("unlock %s", p)
	f := flock.New(p)
	f.Unlock()
	return nil
}
