package object

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
		config *config.T
		paths  BasePaths
	}
)

// List returns the stringified path as data
func (t *Base) List() (string, error) {
	return t.Path.String(), nil
}

func (t *Base) init(path Path) error {
	t.Path = path
	t.log = log.Logger
	t.log = logging.Configure(logging.Config{
		ConsoleLoggingEnabled: true,
		EncodeLogsAsJSON:      true,
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
		Str("sid", config.SessionID).
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
