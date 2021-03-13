package object

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/logging"
)

type (
	// Base is the base struct embedded in all kinded objects.
	Base struct {
		Path     Path
		Volatile bool
		log      zerolog.Logger

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
	t.log = logging.Configure(logging.Config{
		ConsoleLoggingEnabled: true,
		EncodeLogsAsJson:      true,
		FileLoggingEnabled:    true,
		Directory:             t.logDir(),
		Filename:              t.Path.String() + ".log",
		MaxSize:               5,
		MaxBackups:            1,
		MaxAge:                30,
	}).
		With().
		Str("o", t.Path.String()).
		Str("n", config.Node.Hostname).
		Str("sid", config.SessionId).
		Logger()

	if err := t.loadConfig(); err != nil {
		t.log.Debug().Msgf("%s init error: %s", t, err)
		return err
	}
	t.log.Debug().Msgf("%s initialized", t)
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
