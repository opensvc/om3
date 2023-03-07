package object

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/compliance"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/logging"
	"github.com/opensvc/om3/util/xsession"
)

type (
	// core is the base struct embedded in all kinded objects.
	core struct {
		sync.Mutex

		path path.T

		// private
		volatile         bool
		withConsoleLog   bool
		withConsoleColor bool
		log              zerolog.Logger

		// caches
		id         uuid.UUID
		configFile string
		configData any
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

	volatiler interface {
		IsVolatile() bool
		SetVolatile(v bool)
	}

	// Core is implemented by all object kinds.
	Core interface {
		Configurer
		compliancer
		volatiler
		Path() path.T
		FQDN() string
		Status(context.Context) (instance.Status, error)
		FreshStatus(context.Context) (instance.Status, error)
		Nodes() []string
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

func (t *core) init(referrer xconfig.Referrer, id any, opts ...funcopt.O) error {
	if parsed, err := toPathType(id); err != nil {
		return err
	} else {
		t.path = parsed
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		return err
	}
	t.log = logging.Configure(logging.Config{
		ConsoleLoggingEnabled: t.withConsoleLog,
		ConsoleLoggingColor:   t.withConsoleColor,
		EncodeLogsAsJSON:      true,
		FileLoggingEnabled:    !t.volatile,
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

// ConfigFile returns the absolute path of an opensvc object configuration
// file.
func (t core) ConfigFile() string {
	if t.configFile == "" {
		t.configFile = t.path.ConfigFile()
	}
	return t.configFile
}

// Node returns a cache Node struct pointer. If none is already cached,
// allocate a new Node{} and cache it.
func (t *core) Node() (*Node, error) {
	if t.node != nil {
		return t.node, nil
	}
	if n, err := NewNode(); err != nil {
		return nil, err
	} else {
		t.node = n
		return t.node, nil
	}
}

func (t core) Log() *zerolog.Logger {
	return &t.log
}
