package object

import (
	"fmt"
	"path/filepath"
	"time"

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
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/logging"
)

type (
	// Base is the base struct embedded in all kinded objects.
	Base struct {
		Path path.T

		// private
		volatile bool
		log      zerolog.Logger

		// caches
		id        uuid.UUID
		config    *config.T
		node      *Node
		paths     BasePaths
		resources []resource.Driver
	}

	// OptsGlobal contains options accepted by all actions
	OptsGlobal struct {
		Color          string `flag:"color"`
		Format         string `flag:"format"`
		Server         string `flag:"server"`
		Local          bool   `flag:"local"`
		NodeSelector   string `flag:"node"`
		ObjectSelector string `flag:"object"`
		DryRun         bool   `flag:"dry-run"`
	}

	// OptsLocking contains options accepted by all actions using an action lock
	OptsLocking struct {
		Disable bool          `flag:"nolock"`
		Timeout time.Duration `flag:"waitlock"`
	}

	// OptsAsync contains options accepted by all actions having an orchestration
	OptsAsync struct {
		Watch bool          `flag:"watch"`
		Wait  bool          `flag:"wait"`
		Time  time.Duration `flag:"time"`
	}

	// OptsResourceSelector contains options accepted by all actions manipulating resources
	OptsResourceSelector struct {
		ID     string `flag:"rid"`
		Subset string `flag:"subsets"`
		Tag    string `flag:"tags"`
	}
)

// List returns the stringified path as data
func (t *Base) List() (string, error) {
	return t.Path.String(), nil
}

func (t *Base) init(p path.T, opts ...funcopt.O) error {
	t.Path = p
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Debug().Msgf("%s init error: %s", t, err)
		return err
	}
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

func (t Base) IsVolatile() bool {
	return t.volatile
}

func (t *Base) listResources() []resource.Driver {
	if t.resources != nil {
		return t.resources
	}
	t.resources = make([]resource.Driver, 0)
	for _, k := range t.config.SectionStrings() {
		rid := NewResourceID(k)
		if rid.DriverGroup() == drivergroup.Unknown {
			t.log.Debug().Str("rid", k).Msg("unknown driver group")
			continue
		}
		driverGroup := rid.DriverGroup()
		driverName := t.config.GetString(key.New(k, "type"))
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
		t.log.Debug().Str("kw", kw.Option).Msg("")
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

// WithVolatile makes sure not data is ever written by the object.
func WithVolatile(s bool) funcopt.O {
	return funcopt.F(func(t interface{}) error {
		base := t.(*Base)
		base.volatile = s
		return nil
	})
}

// NewFromPath allocates a new kinded object
func NewFromPath(p path.T, opts ...funcopt.O) interface{} {
	switch p.Kind {
	case kind.Svc:
		return NewSvc(p, opts...)
	case kind.Vol:
		return NewVol(p, opts...)
	case kind.Cfg:
		return NewCfg(p, opts...)
	case kind.Sec:
		return NewSec(p, opts...)
	case kind.Usr:
		return NewUsr(p, opts...)
	case kind.Ccfg:
		return NewCcfg(p, opts...)
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

//
// editedConfigFile returns the absolute path of an opensvc object configuration
// file for edition.
//
func (t Base) editedConfigFile() string {
	return t.ConfigFile() + ".tmp"
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
