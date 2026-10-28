package driver

import (
	"fmt"
	"plugin"
	"sort"
)

type (
	Dumper interface {
		Dump() map[ID]any
	}
	Driver struct {
		Allocator any
		Plugin    string

		// Doc is a line saying what the driver does, as a catalog of
		// the drivers shows it.
		Doc string

		// AliasOf is the driver this one is another name of, kept for
		// the configurations written with it.
		AliasOf ID

		// Selects are the drivers a virtual driver picks among on a
		// node, in the order it prefers them.
		Selects IDs
	}
	Registry map[ID]Driver
)

var (
	All = NewRegistry()
)

func NewRegistry() Registry {
	m := make(map[ID]Driver)
	return m
}

func Exists(id ID) bool {
	_, ok := All[id]
	return ok
}

func Get(id ID) (Driver, bool) {
	if drv, ok := All[id]; ok {
		return drv, ok
	}
	// <group>.<name> driver not found, ... try <group>
	// used for example by the volume driver, whose
	// type keyword is not pointing a resource sub driver
	// but a pool driver.
	drv, ok := All[ID{Name: "", Group: id.Group}]
	return drv, ok
}

func GetStrict(id ID) (Driver, bool) {
	drv, ok := All[id]
	return drv, ok
}

// List returns the registered driver ids, in a stable order.
//
// The registry is a map, and what is generated from this list is compared
// from one agent version to the next: the keyword documentation of a release
// is diffed against the previous one, which a listing that comes out in a
// different order every run makes impossible.
func List() IDs {
	l := make(IDs, len(All))
	i := 0
	for did := range All {
		l[i] = did
		i = i + 1
	}
	sort.Sort(l)
	return l
}

func NamesByGroup() map[Group][]string {
	m := make(map[Group][]string)
	for did := range All {
		var l []string
		l, _ = m[did.Group]
		m[did.Group] = append(l, did.Name)
	}
	return m
}

func Register(id ID, allocator any) {
	All[id] = Driver{
		Allocator: allocator,
	}
}

// SetDoc sets the line saying what the registered driver id does.
func SetDoc(id ID, doc string) {
	if drv, ok := All[id]; ok {
		drv.Doc = doc
		All[id] = drv
	}
}

// RegisterAlias registers id as another name of the driver of, which is
// registered already: it takes the allocator and the doc of of.
func RegisterAlias(id, of ID) {
	drv, ok := All[of]
	if !ok {
		return
	}
	All[id] = Driver{
		Allocator: drv.Allocator,
		Plugin:    drv.Plugin,
		Doc:       drv.Doc,
		AliasOf:   of,
	}
}

// SetSelects says the registered virtual driver id picks among the drivers
// ids on a node, in the order it prefers them.
func SetSelects(id ID, ids ...ID) {
	if drv, ok := All[id]; ok {
		drv.Selects = append(IDs{}, ids...)
		All[id] = drv
	}
}

func RegisterFromPlugin(id ID, allocator any, plugin string) {
	All[id] = Driver{
		Allocator: allocator,
		Plugin:    plugin,
	}
}

func (t Registry) WithID(group Group, section string) Registry {
	m := NewRegistry()
	did := NewID(group, section)
	drv, ok := t[did]
	if !ok {
		return m
	}
	m[did] = drv
	return m
}

func (t Registry) WithGroup(group Group) Registry {
	m := NewRegistry()
	for did, drv := range t {
		if did.Group != group {
			continue
		}
		m[did] = drv
	}
	return m
}

func LoadBundle(pluginPath string) error {
	p, err := plugin.Open(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to open plugin at %s: %w", pluginPath, err)
	}

	symbol, err := p.Lookup("Factory")
	if err != nil {
		return fmt.Errorf("failed to lookup symbol 'Factory': %w", err)
	}

	factory, ok := symbol.(Dumper)
	if !ok {
		return fmt.Errorf("symbol 'Factory' loaded from plugin %s is not of type driver.Dumper: %T", pluginPath, symbol)
	}
	for drvID, allocator := range factory.Dump() {
		RegisterFromPlugin(drvID, allocator, pluginPath)
	}
	return nil
}
