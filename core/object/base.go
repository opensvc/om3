package object

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/ssrathi/go-attr"
	"opensvc.com/opensvc/core/actionresdeps"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/driverid"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/core/resourceset"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/logging"
	"opensvc.com/opensvc/util/pg"
	"opensvc.com/opensvc/util/xsession"
)

type (
	// Base is the base struct embedded in all kinded objects.
	Base struct {
		Path path.T
		PG   *pg.Config

		// private
		volatile bool
		log      zerolog.Logger

		// caches
		id                 uuid.UUID
		configFile         string
		config             *xconfig.T
		node               *Node
		paths              BasePaths
		resources          resource.Drivers
		_resources         resource.Drivers
		actionResourceDeps *actionresdeps.Store
		mu                 sync.Mutex

		// method plugs
		postCommit func() error
	}
)

func (t *Base) PostCommit() error {
	if t.postCommit == nil {
		return nil
	}
	return t.postCommit()
}

func (t *Base) SetPostCommit(fn func() error) {
	t.postCommit = fn
}

// List returns the stringified path as data
func (t *Base) List() (string, error) {
	return t.Path.String(), nil
}

func (t *Base) init(referrer xconfig.Referrer, p path.T, opts ...funcopt.O) error {
	t.Path = p
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Debug().Msgf("%s init error: %s", t, err)
		return err
	}
	t.log = logging.Configure(logging.Config{
		ConsoleLoggingEnabled: true,
		EncodeLogsAsJSON:      true,
		FileLoggingEnabled:    true,
		Directory:             t.logDir(), // contains the ns/kind
		Filename:              t.Path.Name + ".log",
		MaxSize:               5,
		MaxBackups:            1,
		MaxAge:                30,
	}).
		With().
		Stringer("o", t.Path).
		Str("n", hostname.Hostname()).
		Str("sid", xsession.ID).
		Logger()

	if err := t.loadConfig(referrer); err != nil {
		t.log.Debug().Msgf("%s init error: %s", t, err)
		return err
	}
	t.PG = t.pgConfig("")
	t.actionResourceDeps = actionresdeps.NewStore()
	t.log.Debug().Msgf("%s initialized", t)
	return nil
}

func (t Base) String() string {
	return t.Path.String()
}

func (t *Base) SetVolatile(v bool) {
	t.volatile = v
}

func (t Base) IsVolatile() bool {
	return t.volatile
}

func (t *Base) ResourceSets() resourceset.L {
	l := resourceset.NewList()
	done := make(map[string]*resourceset.T)
	//
	// subsetSectionString returns the existing section name found in the
	// config file
	//   [subset#fs:g1]   (most precise)
	//   [subset#g1]      (less precise)
	//
	subsetSectionString := func(g drivergroup.T, name string) string {
		s := resourceset.FormatSectionName(g.String(), name)
		if t.config.HasSectionString(s) {
			return s
		}
		return "subset#" + s
	}
	//
	// configureResourceSet allocates and configures the resourceset, looking
	// for keywords in the following sections:
	//   [subset#fs:g1]   (most precise)
	//   [subset#g1]      (less precise)
	//
	// If the rset is already configured, avoid doing the job twice.
	//
	configureResourceSet := func(g drivergroup.T, name string) *resourceset.T {
		id := resourceset.FormatSectionName(g.String(), name)
		if rset, ok := done[id]; ok {
			return rset
		}
		k := subsetSectionString(g, name)
		rset := resourceset.New()
		rset.DriverGroup = g
		rset.Name = name
		rset.SectionName = k
		rset.ResourceLister = t
		parallelKey := key.New(k, "parallel")
		rset.Parallel = t.config.GetBool(parallelKey)
		rset.PG = t.pgConfig(k)
		rset.SetLogger(&t.log)
		done[id] = rset
		l = append(l, rset)
		return rset
	}

	for _, k := range t.config.SectionStrings() {
		//
		// look for resource sections with a defined subset
		//   [fs#1]
		//   subset = g1
		//
		rid, err := resourceid.Parse(k)
		if err != nil {
			continue
		}
		subsetKey := key.New(k, "subset")
		subsetName := t.config.Get(subsetKey)
		if subsetName == "" {
			// discard section with no 'subset' keyword
			continue
		}
		configureResourceSet(rid.DriverGroup(), subsetName)
	}

	// add generic resourcesets not already found as a section
	for _, k := range drivergroup.Names() {
		if _, ok := done[k]; ok {
			continue
		}
		if rset, err := resourceset.Generic(k); err == nil {
			rset.ResourceLister = t
			l = append(l, rset)
		} else {
			t.log.Debug().Err(err)
		}
	}
	sort.Sort(l)
	return l
}

func (t Base) getConfiguringResourceByID(rid string) resource.Driver {
	for _, r := range t._resources {
		if r.RID() == rid {
			return r
		}
	}
	return nil
}

func (t Base) getConfiguredResourceByID(rid string) resource.Driver {
	for _, r := range t.resources {
		if r.RID() == rid {
			return r
		}
	}
	return nil
}

func (t Base) ResourceByID(rid string) resource.Driver {
	if r := t.getConfiguredResourceByID(rid); r != nil {
		return r
	}
	return t.getConfiguringResourceByID(rid)
}

func ResourcesByDrivergroups(i interface{}, drvgrps []drivergroup.T) resource.Drivers {
	t, _ := i.(Baser)
	l := make([]resource.Driver, 0)
	for _, r := range t.Resources() {
		drvgrp := r.ID().DriverGroup()
		for _, d := range drvgrps {
			if drvgrp == d {
				l = append(l, r)
				break
			}
		}
	}
	return resource.Drivers(l)
}

func (t *Base) Resources() resource.Drivers {
	if t.resources != nil {
		return t.resources
	}
	t.configureResources()
	return t.resources
}

func (t *Base) configureResources() {
	t.mu.Lock()
	defer t.mu.Unlock()
	begin := time.Now()
	postponed := make(map[string][]resource.Driver)
	t._resources = make(resource.Drivers, 0)
	for _, k := range t.config.SectionStrings() {
		rid, err := resourceid.Parse(k)
		if err != nil {
			continue
		}
		driverGroup := rid.DriverGroup()
		if driverGroup == drivergroup.Unknown {
			t.log.Debug().Str("rid", k).Str("f", "listResources").Msg("unknown driver group")
			continue
		}
		typeKey := key.New(k, "type")
		driverName := t.config.Get(typeKey)
		driverID := driverid.New(driverGroup, driverName)
		if driverName == "" && driverID.Name == "" {
			t.log.Debug().Stringer("rid", rid).Msg("no explicit type and no default type for this driver group")
			continue
		}
		factory := resource.NewResourceFunc(*driverID)
		if factory == nil {
			t.log.Debug().Stringer("driver", driverID).Msg("driver not found")
			continue
		}
		r := factory()
		rBegin := time.Now()
		if err := t.configureResource(r, k); err != nil {
			switch o := err.(type) {
			case xconfig.ErrPostponedRef:
				if _, ok := postponed[o.RID]; !ok {
					postponed[o.RID] = make([]resource.Driver, 0)
				}
				postponed[o.RID] = append(postponed[o.RID], r)
			default:
				t.log.Error().
					Err(err).
					Str("rid", k).
					Msg("configure resource")
			}
			continue
		}
		t.log.Debug().Str("rid", r.RID()).Dur("duration", time.Now().Sub(rBegin)).Msg("configure resource")
		t._resources = append(t._resources, r)
	}
	for _, resources := range postponed {
		for _, r := range resources {
			if err := t.ReconfigureResource(r); err != nil {
				t.log.Error().
					Err(err).
					Str("rid", r.RID()).
					Msg("configure postponed resource")
				continue
			}
			t.log.Debug().Str("rid", r.RID()).Msgf("configure postponed resource: %+v", r)
			t._resources = append(t._resources, r)
		}
	}
	t.resources = t._resources
	t._resources = nil
	t.log.Debug().Dur("duration", time.Now().Sub(begin)).Msg("configure resources")
	return
}

func (t *Base) ReconfigureResource(r resource.Driver) error {
	return t.configureResource(r, r.RID())
}

func (t *Base) configureResource(r resource.Driver, rid string) error {
	r.SetRID(rid)
	m := r.Manifest()
	for _, kw := range m.Keywords {
		//r.Log().Debug().Str("kw", kw.Option).Msg("")
		k := key.New(rid, kw.Option)
		val, err := t.config.EvalKeywordAs(k, kw, "")
		if err != nil {
			if kw.Required {
				return err
			}
			r.Log().Debug().Msgf("%s keyword eval: %s", k, err)
			continue
		}
		if err := kw.SetValue(r, val); err != nil {
			return errors.Wrapf(err, "%s.%s", rid, kw.Option)
		}
	}
	getDNS := func() ([]string, error) {
		return t.Node().Nameservers()
	}
	getCNIConfig := func() (string, error) {
		return t.Node().CNIConfig()
	}
	getCNIPlugins := func() (string, error) {
		return t.Node().CNIPlugins()
	}

	setAttr := func(c manifest.Context) error {
		switch {
		case c.Ref == "object.path":
			if err := attr.SetValue(r, c.Attr, t.Path); err != nil {
				return err
			}
		case c.Ref == "object.nodes":
			if err := attr.SetValue(r, c.Attr, t.Nodes()); err != nil {
				return err
			}
		case c.Ref == "object.peers":
			if err := attr.SetValue(r, c.Attr, t.Peers()); err != nil {
				return err
			}
		case c.Ref == "object.id":
			if err := attr.SetValue(r, c.Attr, t.ID()); err != nil {
				return err
			}
		case c.Ref == "object.topology":
			if err := attr.SetValue(r, c.Attr, t.Topology()); err != nil {
				return err
			}
		case c.Ref == "node.dns":
			if dns, err := getDNS(); err != nil {
				return err
			} else if err := attr.SetValue(r, c.Attr, dns); err != nil {
				return err
			}
		case c.Ref == "cni.config":
			if s, err := getCNIConfig(); err != nil {
				return err
			} else if err := attr.SetValue(r, c.Attr, s); err != nil {
				return err
			}
		case c.Ref == "cni.plugins":
			if s, err := getCNIPlugins(); err != nil {
				return err
			} else if err := attr.SetValue(r, c.Attr, s); err != nil {
				return err
			}
		}
		return nil
	}
	for _, c := range m.Context {
		if err := setAttr(c); err != nil {
			return errors.Wrapf(err, "%s", c.Attr)
		}
	}
	r.SetObject(t)
	r.SetPG(t.pgConfig(rid))
	if i, ok := r.(resource.ActionResourceDepser); ok {
		deps := i.ActionResourceDeps()
		t.actionResourceDeps.RegisterSlice(deps)
	}
	r.Log().Debug().Msgf("configured resource: %+v", r)
	return nil
}

func (t Base) GetActionResDeps() *actionresdeps.Store {
	return t.actionResourceDeps
}

//
// ConfigFile returns the absolute path of an opensvc object configuration
// file.
//
func (t Base) ConfigFile() string {
	if t.configFile == "" {
		t.configFile = t.standardConfigFile()
	}
	return t.configFile
}

//
// SetStandardConfigFile changes the configuration file currently set
// usually by NewFromPath(..., WithConfigFile(fpath), ...) with the
// standard configuration file location.
//
func (t Base) SetStandardConfigFile() {
	t.configFile = t.standardConfigFile()
}

func (t Base) standardConfigFile() string {
	p := t.Path.String()
	switch t.Path.Namespace {
	case "", "root":
		p = fmt.Sprintf("%s/%s.conf", rawconfig.Node.Paths.Etc, p)
	default:
		p = fmt.Sprintf("%s/%s.conf", rawconfig.Node.Paths.EtcNs, p)
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
		p = fmt.Sprintf("%s/%s/%s", rawconfig.Node.Paths.Var, t.Path.Kind, t.Path.Name)
	default:
		p = fmt.Sprintf("%s/namespaces/%s", rawconfig.Node.Paths.Var, p)
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
		p = fmt.Sprintf("%s/namespaces/%s/%s", rawconfig.Node.Paths.Tmp, t.Path.Namespace, t.Path.Kind)
	case t.Path.Kind == kind.Svc, t.Path.Kind == kind.Ccfg:
		p = fmt.Sprintf("%s", rawconfig.Node.Paths.Tmp)
	default:
		p = fmt.Sprintf("%s/%s", rawconfig.Node.Paths.Tmp, t.Path.Kind)
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
		p = fmt.Sprintf("%s/namespaces/%s/%s", rawconfig.Node.Paths.Log, t.Path.Namespace, t.Path.Kind)
	case t.Path.Kind == kind.Svc, t.Path.Kind == kind.Ccfg:
		p = fmt.Sprintf("%s", rawconfig.Node.Paths.Log)
	default:
		p = fmt.Sprintf("%s/%s", rawconfig.Node.Paths.Log, t.Path.Kind)
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

func (t Base) Log() *zerolog.Logger {
	return &t.log
}

// IsDesc is a requirement of the ResourceLister interface. Base Resources() is always ascending.
func (t *Base) IsDesc() bool {
	return false
}
