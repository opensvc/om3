package object

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/compliance"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/logging"
	"opensvc.com/opensvc/util/xsession"
)

type (
	// core is the base struct embedded in all kinded objects.
	core struct {
		sync.Mutex

		path path.T

		// private
		volatile bool
		log      zerolog.Logger

		// caches
		id         uuid.UUID
		configFile string
		configData []byte
		config     *xconfig.T
		node       *Node
		paths      struct {
			varDir string
			logDir string
			tmpDir string
		}

		// method plugs
		postCommit func() error
	}

	compliancer interface {
		NewCompliance() (*compliance.T, error)
	}

	// corer is implemented by all object kinds.
	corer interface {
		Configurer
		compliancer
		Path() path.T
		FQDN() string
		IsVolatile() bool
		SetVolatile(v bool)
		Status(context.Context) (instance.Status, error)
		FreshStatus(context.Context) (instance.Status, error)
	}
)

func (t *core) PostCommit() error {
	if t.postCommit == nil {
		return nil
	}
	return t.postCommit()
}

func (t *core) SetPostCommit(fn func() error) {
	t.postCommit = fn
}

// List returns the stringified path as data
func (t *core) List() (string, error) {
	return t.path.String(), nil
}

func (t *core) init(referrer xconfig.Referrer, p path.T, opts ...funcopt.O) error {
	t.path = p
	if err := funcopt.Apply(t, opts...); err != nil {
		return err
	}
	t.log = logging.Configure(logging.Config{
		ConsoleLoggingEnabled: zerolog.GlobalLevel() == zerolog.DebugLevel,
		EncodeLogsAsJSON:      true,
		FileLoggingEnabled:    true,
		Directory:             t.logDir(), // contains the ns/kind
		Filename:              t.path.Name + ".log",
		MaxSize:               5,
		MaxBackups:            1,
		MaxAge:                30,
	}).
		With().
		Stringer("o", t.path).
		Str("n", hostname.Hostname()).
		Str("sid", xsession.ID).
		Logger()

	if err := t.loadConfig(referrer); err != nil {
		return err
	}
	return nil
}

func (t core) String() string {
	return t.path.String()
}

func (t *core) SetVolatile(v bool) {
	t.volatile = v
}

func (t core) IsVolatile() bool {
	return t.volatile
}

func (t *core) Path() path.T {
	return t.path
}

//
// ConfigFile returns the absolute path of an opensvc object configuration
// file.
//
func (t core) ConfigFile() string {
	if t.configFile == "" {
		t.configFile = t.path.ConfigFile()
	}
	return t.configFile
}

//
// Node returns a cache Node struct pointer. If none is already cached,
// allocate a new Node{} and cache it.
//
func (t *core) Node() *Node {
	if t.node != nil {
		return t.node
	}
	t.node = NewNode()
	return t.node
}

func (t core) Log() *zerolog.Logger {
	return &t.log
}
