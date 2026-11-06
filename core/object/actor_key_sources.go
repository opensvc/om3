package object

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/waitfor"
)

type (
	// keySource is a key a keyword of a resource section seeds from a
	// source, with the keyword naming it.
	keySource struct {
		keywords.KeySource
		Keyword key.T
	}
)

var (
	// randomKeyWaitTimeout is how long the provision of a node other than
	// the leader waits for a key the leader generates to reach it, with the
	// configuration of its datastore: the leader provisions first, and the
	// daemon brings the configurations it writes to the other nodes a
	// moment later.
	randomKeyWaitTimeout = time.Minute

	// randomKeyWaitInterval is how often the node looks for the keys it
	// waits for, in the configurations it holds.
	randomKeyWaitInterval = time.Second

	keySourceOptionsCache   = make(map[driver.ID][]string)
	keySourceOptionsCacheMu sync.Mutex
)

// keySourceOptions returns the options of the keywords of the drivers of a
// section, of the group and type given, that name keys with a source: the
// drivers the keyword lookup asks, a section naming no type being a resource
// of the default driver of its group.
func keySourceOptions(group driver.Group, sectionType string) []string {
	if group == driver.GroupVolume {
		sectionType = ""
	}
	id := driver.ID{Group: group, Name: sectionType}
	keySourceOptionsCacheMu.Lock()
	defer keySourceOptionsCacheMu.Unlock()
	if l, ok := keySourceOptionsCache[id]; ok {
		return l
	}
	var drivers driver.Registry
	if sectionType != "" || driver.DefaultDriver[group] != "" {
		drivers = driver.All.WithID(group, sectionType)
	} else {
		drivers = driver.All.WithGroup(group)
	}
	l := make([]string, 0)
	for _, i := range drivers {
		allocator, ok := i.Allocator.(func() resource.Driver)
		if !ok {
			continue
		}
		for _, kw := range manifest.Get(allocator()).Keywords() {
			if kw.KeySources != nil && !slices.Contains(l, kw.Option) {
				l = append(l, kw.Option)
			}
		}
	}
	sort.Strings(l)
	keySourceOptionsCache[id] = l
	return l
}

// keySourcesAs returns the keys the resource sections of the configuration
// seed from a source, their keywords evaluated as the node given: a keyword
// a section inherits from DEFAULT, or reading another keyword by reference,
// is read as the section evaluates it, the relative datastore paths being of
// the namespace given.
//
// A value that does not evaluate names no key: the validation of the keyword
// reports it. A value whose source the grammar of its keyword refuses names
// none either, and is returned as a refusal, for the validation to report,
// the resource refusing it at its action too.
func keySourcesAs(cfg *xconfig.T, namespace, nodename string) ([]keySource, []keySourceRefusal) {
	l := make([]keySource, 0)
	refusals := make([]keySourceRefusal, 0)
	if cfg == nil || cfg.Referrer == nil {
		return l, refusals
	}
	for _, section := range cfg.SectionStrings() {
		rid, err := resourceid.Parse(section)
		if err != nil || rid.DriverGroup() == driver.GroupUnknown {
			continue
		}
		sectionType := cfg.GetString(key.New(section, "type"))
		for _, option := range keySourceOptions(rid.DriverGroup(), sectionType) {
			k := key.New(section, option)
			kw := cfg.Referrer.KeywordLookup(k, sectionType)
			if kw == nil || kw.KeySources == nil {
				continue
			}
			v, err := cfg.EvalKeywordAs(k, kw, nodename)
			if err != nil {
				continue
			}
			value, _ := v.([]string)
			if len(value) == 0 {
				continue
			}
			sources, err := kw.KeySources(value, namespace)
			if err != nil {
				refusals = append(refusals, keySourceRefusal{Keyword: k, Err: err})
				continue
			}
			for _, s := range sources {
				l = append(l, keySource{KeySource: s, Keyword: k})
			}
		}
	}
	return l, refusals
}

// keySourceRefusal is a keyword whose source its grammar refuses.
type keySourceRefusal struct {
	Keyword key.T
	Err     error
}

// keySourceNodes returns the nodes a configuration is evaluated as to read
// the keys it seeds: this node, the nodes and drpnodes of the object, and
// the nodes a keyword is scoped to.
func keySourceNodes(cfg *xconfig.T) []string {
	l := []string{hostname.Hostname()}
	add := func(nodes ...string) {
		for _, nodename := range nodes {
			if nodename != "" && !slices.Contains(l, nodename) {
				l = append(l, nodename)
			}
		}
	}
	for _, option := range []string{"nodes", "drpnodes"} {
		if v, err := cfg.Eval(key.New("DEFAULT", option)); err == nil {
			if nodes, ok := v.([]string); ok {
				add(nodes...)
			}
		}
	}
	for _, section := range cfg.SectionStrings() {
		for _, option := range cfg.Keys(section) {
			_, scope, ok := strings.Cut(option, "@")
			if !ok {
				continue
			}
			switch scope {
			case "nodes", "drpnodes", "encapnodes":
			default:
				add(scope)
			}
		}
	}
	return l
}

// ValidateSections checks the keys the resource sections of the
// configuration seed from a source, on every node of the object:
//
//   - a key two keywords seed from two sources is refused: which one seeds it
//     would depend on the order the resources act in, and on the node that
//     acts first. Where the keywords are written, in a section, in DEFAULT,
//     or read by a reference, does not matter: each is read as its section
//     evaluates it.
//
//   - a key of a datastore of another namespace is refused: a datastore
//     sharing its keys with the namespace of the object lets it read them,
//     and writing one is not the reader's to do.
func (t *actor) ValidateSections(cfg *xconfig.T) xconfig.Alerts {
	type seen struct {
		source  string
		keyword key.T
	}
	alerts := make(xconfig.Alerts, 0)
	first := make(map[string]seen)
	reported := make(map[string]bool)
	for _, nodename := range keySourceNodes(cfg) {
		sources, refusals := keySourcesAs(cfg, t.path.Namespace, nodename)
		for _, r := range refusals {
			id := r.Keyword.String() + " " + r.Err.Error()
			if !reported[id] {
				reported[id] = true
				alerts = append(alerts, cfg.NewAlertKeySource(r.Keyword, sectionDriverID(cfg, r.Keyword.Section), r.Err.Error()))
			}
		}
		for _, ks := range sources {
			id := ks.Path.String() + " key " + ks.Key
			if ks.Path.Namespace != t.path.Namespace {
				foreignID := ks.Keyword.String() + " " + id
				if !reported[foreignID] {
					reported[foreignID] = true
					comment := fmt.Sprintf("%s key %s source %s: %s, refused for every source", ks.Path, ks.Key, ks.Source, ErrSeedOtherNamespace)
					alerts = append(alerts, cfg.NewAlertKeySource(ks.Keyword, sectionDriverID(cfg, ks.Keyword.Section), comment))
				}
				continue
			}
			f, ok := first[id]
			if !ok {
				first[id] = seen{source: ks.Source, keyword: ks.Keyword}
				continue
			}
			if f.source == ks.Source || reported[id] {
				continue
			}
			reported[id] = true
			comment := fmt.Sprintf("%s is seeded from source %s by %s and from source %s by %s: a key has one source", id, f.source, f.keyword, ks.Source, ks.Keyword)
			alerts = append(alerts, cfg.NewAlertKeySource(ks.Keyword, sectionDriverID(cfg, ks.Keyword.Section), comment))
		}
	}
	return alerts
}

// sectionDriverID returns the driver of a resource section, as the
// validation of its keywords names it.
func sectionDriverID(cfg *xconfig.T, section string) driver.ID {
	rid, err := resourceid.Parse(section)
	if err != nil {
		return driver.ID{}
	}
	return driver.NewID(rid.DriverGroup(), cfg.GetString(key.New(section, "type")))
}

// seedKeySources writes the keys the enabled resources of the object seed
// from a source, which their datastore does not hold, before the resources
// act: an environment reads its variables as the container or the process
// starts, and an install its files as the volume starts.
//
// A key already there is never written: a key seeded is the object's from
// then on, and its value what the data it protects was made with. The
// datastores of the namespace of the object are created as needed, and the
// keys of the datastores of another namespace refused.
//
// A key generated, "source random", is written by the provision of the
// leader node, once for all the nodes. The provision of another node waits
// for it to reach the node, a bounded while, and fails without it: a key
// made there would differ from the one the leader made. A start generates
// none, the resource reading it failing on its absence.
//
// The other sources are read at each provision and start, as the install
// lines read them: a source failing is warned of, and the resource reading
// the key goes without it, as it does with any key missing.
func (t *actor) seedKeySources(ctx context.Context, provision bool) error {
	leader := actioncontext.IsLeader(ctx)
	enabled := make(map[string]bool)
	for _, r := range t.Resources() {
		if !r.IsDisabled() {
			enabled[r.RID()] = true
		}
	}
	var (
		errs    []error
		pending []keySource
	)
	fail := func(err error) {
		if provision {
			errs = append(errs, err)
		} else {
			t.log.Warnf("%s", err)
		}
	}
	done := make(map[string]bool)
	sources, _ := keySourcesAs(t.config, t.path.Namespace, hostname.Hostname())
	for _, ks := range sources {
		if !enabled[ks.Keyword.Section] {
			continue
		}
		id := ks.Path.String() + " key " + ks.Key
		if done[id] {
			continue
		}
		done[id] = true
		if HasDataStoreKey(ks.Path, ks.Key) {
			continue
		}
		if ks.Path.Namespace != t.path.Namespace {
			fail(fmt.Errorf("%s: %s: %w", ks.Keyword, id, ErrSeedOtherNamespace))
			continue
		}
		if ks.IsRandom() {
			switch {
			case !provision:
				continue
			case !leader:
				pending = append(pending, ks)
				continue
			}
		}
		ds, created, err := EnsureDataStore(ks.Path, t.path.Namespace)
		if err != nil {
			fail(fmt.Errorf("%s: %w", ks.Keyword, err))
			continue
		}
		if created {
			t.log.Infof("%s: created datastore %s for seeding", ks.Keyword, ks.Path)
		}
		if err := SeedKey(ctx, ds, ks.Key, ks.Source, t.log, t.path, ""); err != nil {
			err = fmt.Errorf("%s: failed to seed %s from source %s: %w", ks.Keyword, id, ks.Source, err)
			if ks.IsRandom() {
				errs = append(errs, err)
			} else {
				t.log.Warnf("%s", err)
			}
		}
	}
	if err := t.waitRandomKeys(ctx, pending); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// waitRandomKeys waits for the keys the provision of the leader node
// generates to reach this node, with the configuration of their datastore,
// and fails naming the ones that did not.
func (t *actor) waitRandomKeys(ctx context.Context, l []keySource) error {
	if len(l) == 0 {
		return nil
	}
	missing := func() []keySource {
		m := make([]keySource, 0)
		for _, ks := range l {
			if !HasDataStoreKey(ks.Path, ks.Key) {
				m = append(m, ks)
			}
		}
		return m
	}
	for _, ks := range l {
		t.log.Infof("%s: wait for the key %s of %s the provision of the leader node generates", ks.Keyword, ks.Key, ks.Path)
	}
	if waitfor.TrueCtx(ctx, randomKeyWaitTimeout, randomKeyWaitInterval, func() bool { return len(missing()) == 0 }) {
		return nil
	}
	var errs []error
	for _, ks := range missing() {
		errs = append(errs, fmt.Errorf("%s: waited %s: %w", ks.Keyword, randomKeyWaitTimeout, ErrRandomKeyMissing{Path: ks.Path, Key: ks.Key}))
	}
	return errors.Join(errs...)
}
