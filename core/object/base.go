package object

import (
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/ssrathi/go-attr"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/logging"
)

type (
	// Base is the base struct embedded in all kinded objects.
	Base struct {
		Path     path.T
		Volatile bool
		log      zerolog.Logger

		// caches
		id        uuid.UUID
		config    *config.T
		node      *Node
		paths     BasePaths
		resources []resource.Driver
	}
)

// List returns the stringified path as data
func (t *Base) List() (string, error) {
	return t.Path.String(), nil
}

func (t *Base) init(p path.T) error {
	t.Path = p
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

func (t *Base) listResources() []resource.Driver {
	if t.resources != nil {
		return t.resources
	}
	t.resources = make([]resource.Driver, 0)
	for k, _ := range t.config.Raw() {
		rid := NewResourceID(k)
		if rid.DriverGroup() == drivergroup.Unknown {
			t.log.Debug().Str("rid", k).Msg("unknown driver group")
			continue
		}
		driverGroup := rid.DriverGroup()
		driverName := t.config.GetStringP(k, "type")
		driverID := resource.NewDriverID(driverGroup, driverName)
		factory := driverID.NewResourceFunc()
		if factory == nil {
			t.log.Debug().Str("driver", driverID.String()).Msg("driver not found")
			continue
		}
		r := factory()
		if err := t.configureResource(r, k); err != nil {
			t.log.Error().
				Err(err).
				Str("rid", k).
				Msg("configureResource")
			continue
		}
		t.resources = append(t.resources, r)
	}
	return t.resources
}

func (t Base) configureResource(r resource.Driver, rid string) error {
	r.SetRID(rid)
	m := r.Manifest()
	for _, kw := range m.Keywords {
		t.log.Debug().Str("kw", kw.Name).Msg("")
	}
	for _, c := range m.Context {
		switch {
		case c.Ref == "object.path":
			if err := attr.SetValue(r, c.Attr, t.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

// NewFromPath allocates a new kinded object
func NewFromPath(p path.T) interface{} {
	switch p.Kind {
	case kind.Svc:
		return NewSvc(p)
	case kind.Vol:
		return NewVol(p)
	case kind.Cfg:
		return NewCfg(p)
	case kind.Sec:
		return NewSec(p)
	case kind.Usr:
		return NewUsr(p)
	case kind.Ccfg:
		return NewCcfg(p)
	default:
		return nil
	}
}

// NewBaserFromPath returns a Baser interface from an object path
func NewBaserFromPath(p path.T) Baser {
	return NewFromPath(p).(Baser)
}

// NewConfigurerFromPath returns a Configurer interface from an object path
func NewConfigurerFromPath(p path.T) Configurer {
	return NewFromPath(p).(Configurer)
}

//
// ConfigFile returns the absolute path of an opensvc object configuration
// file.
//
func (t Base) ConfigFile() string {
	p := t.Path.String()
	switch t.Path.Namespace {
	case "", "root":
		p = fmt.Sprintf("%s/%s.conf", config.Node.Paths.Etc, p)
	default:
		p = fmt.Sprintf("%s/%s.conf", config.Node.Paths.EtcNs, p)
	}
	return filepath.FromSlash(p)
}

// Exists returns true if the object configuration file exists.
func (t Base) Exists() bool {
	return file.Exists(t.ConfigFile())
}

//
// VarDir returns the directory on the local filesystem where the object
// variable persistent data is stored as files.
//
func (t Base) VarDir() string {
	p := t.Path.String()
	switch t.Path.Namespace {
	case "", "root":
		p = fmt.Sprintf("%s/%s/%s", config.Node.Paths.Var, t.Path.Kind, t.Path.Name)
	default:
		p = fmt.Sprintf("%s/namespaces/%s", config.Node.Paths.Var, p)
	}
	return filepath.FromSlash(p)
}

//
// TmpDir returns the directory on the local filesystem where the object
// stores its temporary files.
//
func (t Base) TmpDir() string {
	p := t.Path.String()
	switch {
	case t.Path.Namespace != "", t.Path.Namespace != "root":
		p = fmt.Sprintf("%s/namespaces/%s/%s", config.Node.Paths.Tmp, t.Path.Namespace, t.Path.Kind)
	case t.Path.Kind == kind.Svc, t.Path.Kind == kind.Ccfg:
		p = fmt.Sprintf("%s", config.Node.Paths.Tmp)
	default:
		p = fmt.Sprintf("%s/%s", config.Node.Paths.Tmp, t.Path.Kind)
	}
	return filepath.FromSlash(p)
}

//
// LogDir returns the directory on the local filesystem where the object
// stores its temporary files.
//
func (t Base) LogDir() string {
	p := t.Path.String()
	switch {
	case t.Path.Namespace != "", t.Path.Namespace != "root":
		p = fmt.Sprintf("%s/namespaces/%s/%s", config.Node.Paths.Log, t.Path.Namespace, t.Path.Kind)
	case t.Path.Kind == kind.Svc, t.Path.Kind == kind.Ccfg:
		p = fmt.Sprintf("%s", config.Node.Paths.Log)
	default:
		p = fmt.Sprintf("%s/%s", config.Node.Paths.Log, t.Path.Kind)
	}
	return filepath.FromSlash(p)
}

//
// Node returns a cache Node struct pointer. If none is already cached,
// allocate a new Node{} and cache it.
//
func (t *Base) Node() *Node {
	if t.node != nil {
		return t.node
	}
	t.node = NewNode()
	return t.node
}
