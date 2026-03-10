package manifest

import (
	"context"
	"sync"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
)

type (
	//
	// T describes a driver so callers can format the input as the
	// driver expects.
	//
	// A typical allocation is:
	// m := New("fs", "flag").AddKeyword(kws...).AddContext(ctx...)
	//
	T struct {
		DriverID driver.ID `json:"driver"`
		Kinds    naming.Kinds
		Attrs    map[string]Attr
	}

	//
	// Context is a key-value the resource expects to find in the input,
	// merged with keywords coming from configuration file.
	//
	// For example, a driver often needs the parent object Path, which
	// can be asked via:
	//
	// T{
	//     Context: []Context{
	//         {
	//             Key: "path",
	//             Ref:"object.path",
	//         },
	//     },
	// }
	//
	Context struct {
		// Key is the name of the key in the json representation of the context.
		Key string

		// Attr is the name of the field in the resource struct.
		Attr string

		// Ref is the code describing what context information to embed in the resource struct.
		Ref string
	}

	Attr interface {
		Name() string
	}

	tcache struct {
		sync.RWMutex
		m map[string]*T
	}
)

type (
	provisioner interface {
		Provision(context.Context) error
	}
	unprovisioner interface {
		Unprovision(context.Context) error
	}
	starter interface {
		Start(context.Context) error
	}
	stopper interface {
		Stop(context.Context) error
	}
	runner interface {
		Run(context.Context) error
	}
	syncer interface {
		Update(context.Context) error
	}
)

var (
	cache tcache
)

func init() {
	cache = tcache{
		m: make(map[string]*T),
	}
}

func (t Context) Name() string {
	return t.Attr
}

// AddInterfacesKeywords adds keywords from interfaces and returns t
//
// When interfaces contains both value and pointer receiver r should be a pointer
func (t *T) AddInterfacesKeywords(r any) *T {
	if _, ok := r.(starter); ok {
		t.AddKeywords(starterKeywords...)
	}
	if _, ok := r.(stopper); ok {
		t.AddKeywords(stopperKeywords...)
	}
	if _, ok := r.(provisioner); ok {
		t.AddKeywords(provisionerKeywords...)
	}
	if _, ok := r.(unprovisioner); ok {
		t.AddKeywords(unprovisionerKeywords...)
	}
	if _, ok := r.(syncer); ok {
		t.AddKeywords(syncerKeywords...)
	}
	if _, ok := r.(runner); ok {
		t.AddKeywords(runnerKeywords...)
	}
	return t
}

// New returns *T with keywords defined
//
// It adds generic keywords + keywords from interface keywords.
func New(did driver.ID, r any) *T {
	t := &T{
		DriverID: did,
		Attrs:    make(map[string]Attr),
		Kinds:    make(naming.Kinds),
	}
	t.AddKeywords(genericKeywords...)
	t.AddInterfacesKeywords(r)
	return t
}

// Add dedups the attribute providers
func (t *T) Add(attrs ...Attr) *T {
	for _, attr := range attrs {
		t.Attrs[attr.Name()] = attr
	}
	return t
}

func (t *T) AddKeywords(attrs ...*keywords.Keyword) *T {
	for _, attr := range attrs {
		t.Attrs[attr.Name()] = attr
	}
	return t
}

func (t *T) Keywords() []*keywords.Keyword {
	n := 0
	for _, attr := range t.Attrs {
		if _, ok := attr.(*keywords.Keyword); ok {
			n++
		}
	}
	l := make([]*keywords.Keyword, n)
	n = 0
	for _, attr := range t.Attrs {
		if o, ok := attr.(*keywords.Keyword); ok {
			l[n] = o
			n++
		}
	}
	return l
}

type manifester interface {
	Manifest() *T
	DriverID() driver.ID
}

// Get provides a manifest from a cache indexed by driver id.
// If the cache
func Get(r manifester) *T {
	cache.Lock()
	defer cache.Unlock()
	key := r.DriverID().String()
	if m, ok := cache.m[key]; ok {
		return m
	}
	m := r.Manifest()
	cache.m[key] = m
	return m
}
