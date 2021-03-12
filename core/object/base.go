package object

import (
	"context"
	"errors"
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
func (t *Base) Status(refresh bool) error {
	return nil
}

// List returns the stringified path as data
func (t *Base) List() (string, error) {
	return t.Path.String(), nil
}

// Start starts the local instance of the object
func (t *Base) Start(options ActionOptionsStart) error {
	if err := t.Lock("", options.LockTimeout); err != nil {
		return err
	}
	defer t.Unlock("")
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

// VarDir is the directory hosting the object's variable files
func (t *Base) VarDir() string {
	if t.varDir != "" {
		return t.varDir
	}
	t.varDir = t.Path.VarDir()
	if !t.Volatile {
		if err := os.MkdirAll(t.varDir, os.ModePerm); err != nil {
			log.Error(err)
		}
	}
	return t.varDir
}

// LockFile is the path of the file to use as an action lock.
func (t *Base) LockFile(group string) string {
	p := filepath.Join(t.VarDir(), "lock.generic")
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
func (t *Base) Lock(group string, timeout time.Duration) error {
	p := t.LockFile(group)
	log.Debugf("locking %s, timeout %s", p, timeout)
	f := flock.New(p)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := f.TryLockContext(ctx, 500*time.Millisecond)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return errors.New("lock timeout exceeded")
		}
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
func (t *Base) Unlock(group string) error {
	p := t.LockFile(group)
	log.Debugf("unlock %s", p)
	f := flock.New(p)
	f.Unlock()
	return nil
}
