package netrule

import (
	"errors"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
)

type (
	// Data is a view held as data: what the daemon is to hand to a driver
	// asking for it, and what the tests resolve over.
	Data struct {
		// Networks are the cluster networks, by name.
		Networks map[string]DataNetwork `json:"networks"`

		// NodeAddrs are the addresses of the cluster nodes, by name: a
		// node with none is still a node of the cluster.
		NodeAddrs map[string][]netip.Addr `json:"nodes"`

		// Nameservers are the nameservers what is filtered resolves
		// through.
		Nameservers []netip.Addr `json:"dns"`

		// Objects are the addresses of the objects of the cluster, by
		// path: an object with none is still an object of the cluster.
		Objects Objects `json:"objects,omitempty"`

		// Keys are the keys of the datastores the node holds, decoded,
		// by path then by key name.
		Keys map[string]map[string]string `json:"keys,omitempty"`

		// Shares are the namespaces the datastores share their keys
		// with, beside their own, by path, as their share keyword
		// says them: * is every namespace.
		Shares map[string][]string `json:"shares,omitempty"`

		// Unsure are the paths of the objects with an instance whose
		// status nothing tells for now.
		Unsure []string `json:"unsure,omitempty"`
	}

	// ObjectAddrs are the addresses the resources of an object publish
	// in their status, by resource id, the instances of all the nodes
	// together: an address is the one of the object wherever it runs, so
	// a failover changes nothing of it.
	ObjectAddrs map[string][]netip.Addr

	// Objects are the addresses of objects, by path.
	Objects map[string]ObjectAddrs

	// Identity is which object a path names: its id, and the version
	// of its configuration, each zero when not known. See Holder.
	Identity struct {
		ObjectID string
		Version  time.Time

		// Publishers are the objects publishing the addresses of the
		// path, by address: a path names two objects while a node still
		// tells its instance of the one deleted beside the one created
		// again, each publishing its own addresses.
		//
		// Nil when the daemon tells the object of the path alone, as one
		// not upgraded yet: every address is then the one of that
		// object. Not nil, an address it does not hold is one no
		// configuration of the instances publishing it is known of, and
		// no object is told of it, not the one of the path.
		Publishers map[netip.Addr]Publisher
	}

	// Publisher is the object publishing an address of a path: its id,
	// and the version of its configuration, the newest of the instances
	// publishing the address.
	Publisher struct {
		ObjectID string
		Version  time.Time
	}

	// Identities are which objects the paths name, and which publish
	// their addresses, by path, as the daemon tells them with the
	// addresses of the objects: a path it tells nothing of, as every
	// path told by a daemon knowing no identity, is not there.
	Identities map[string]Identity

	// DataNetwork is a cluster network of a Data view.
	DataNetwork struct {
		// Ranges are the address ranges of the network.
		Ranges []netip.Prefix `json:"ranges"`

		// NodeRanges are the parts of the network the nodes hand out, by
		// node name.
		NodeRanges map[string][]netip.Prefix `json:"node_ranges,omitempty"`
	}
)

// ErrNoLocalView is the error of a binary knowing nothing of the node it
// runs on, as one driving a cluster through its api.
var ErrNoLocalView = errors.New("no view of the cluster networks and nodes here")

// ErrNoDatastore and ErrNoKey are the errors of a view asked a key of a
// datastore the node does not hold, and a key the datastore does not. See
// ErrNotShared for a datastore of another namespace.
var (
	ErrNoDatastore = errors.New("no such datastore")
	ErrNoKey       = errors.New("no such key")
)

// ErrNoObjects is the error of a binary with nobody to ask the addresses of
// the objects.
var ErrNoObjects = errors.New("nobody to ask the addresses of the objects here")

var (
	localViewMu sync.RWMutex
	localView   func() (View, error)
	lookup      func(objects []naming.Path, namespaces []string) (Objects, Identities, error)
)

// RegisterObjects sets the function returning the addresses of the objects
// named and of the objects of the namespaces named, and which objects they
// are, as the daemon of the node knows them: asked through its api by a
// command, and read from its own data by the daemon.
func RegisterObjects(fn func(objects []naming.Path, namespaces []string) (Objects, Identities, error)) {
	localViewMu.Lock()
	defer localViewMu.Unlock()
	lookup = fn
}

// LookupObjects returns the addresses of the objects named and of the
// objects of the namespaces named, an object the cluster does not have
// left out, with which objects they are, and ErrNoObjects where nobody is
// registered to tell.
func LookupObjects(objects []naming.Path, namespaces []string) (Objects, Identities, error) {
	localViewMu.RLock()
	fn := lookup
	localViewMu.RUnlock()
	if fn == nil {
		return nil, nil, ErrNoObjects
	}
	return fn(objects, namespaces)
}

// RegisterLocalView sets the function returning the view of the node the
// caller runs on, read from its configuration.
//
// It is registered rather than called because the package reading the
// networks of a node configuration imports the one validating the object
// configurations, which warns of the rules naming a network or a node the
// cluster does not have.
func RegisterLocalView(fn func() (View, error)) {
	localViewMu.Lock()
	defer localViewMu.Unlock()
	localView = fn
}

// LocalView returns the view of the node the caller runs on, and
// ErrNoLocalView where none is registered.
func LocalView() (View, error) {
	localViewMu.RLock()
	fn := localView
	localViewMu.RUnlock()
	if fn == nil {
		return nil, ErrNoLocalView
	}
	return fn()
}

// dnsView is a view whose nameservers are the ones of a resource.
type dnsView struct {
	View
	dns []netip.Addr
}

func (t dnsView) DNS() []netip.Addr {
	return t.dns
}

// Identity implements Identifier, as the view it stands for does.
func (t dnsView) Identity(path naming.Path) (string, time.Time) {
	return identity(t.View, path)
}

// AddrIdentity implements AddrIdentifier, as the view it stands for does.
func (t dnsView) AddrIdentity(path naming.Path, addr netip.Addr) (string, time.Time) {
	return addrIdentity(t.View, path, addr)
}

// WithDNS returns the view with the nameservers given in place of its own:
// the networks and the nodes are the same for every resource of a node, and
// the nameservers are the ones of the resolver of each, which its driver
// knows.
func WithDNS(view View, dns []netip.Addr) View {
	return dnsView{View: view, dns: dns}
}

// frozenView is a view answering the same of an object, and of a namespace,
// however many times it is asked.
type frozenView struct {
	View
	objects    map[naming.Path]frozenObject
	namespaces map[string][]naming.Path
	keys       map[frozenKeyID]frozenKey
	publishers map[frozenAddrID]Identity
}

type frozenAddrID struct {
	path naming.Path
	addr netip.Addr
}

type frozenKeyID struct {
	datastore naming.Path
	key       string
	namespace string
}

type frozenKey struct {
	value []byte
	err   error
}

type frozenObject struct {
	addrs ObjectAddrs
	ok    bool

	// id and version are which object the path named when its
	// addresses were asked: the addresses held are the ones of that
	// object.
	id      string
	version time.Time
}

// freeze returns the view remembering what it answers of the objects.
//
// The rules of a resource are gone over more than once to make its filter,
// and the view of a daemon reads data that changes meanwhile: an object
// found with an address, then without, made a filter no single state of the
// cluster gives, and a memory of it disagreeing with the filter.
func freeze(view View) View {
	return &frozenView{
		View:       view,
		objects:    make(map[naming.Path]frozenObject),
		namespaces: make(map[string][]naming.Path),
		keys:       make(map[frozenKeyID]frozenKey),
		publishers: make(map[frozenAddrID]Identity),
	}
}

func (t *frozenView) Key(datastore naming.Path, key, namespace string) ([]byte, error) {
	id := frozenKeyID{datastore: datastore, key: key, namespace: namespace}
	k, ok := t.keys[id]
	if !ok {
		k.value, k.err = t.View.Key(datastore, key, namespace)
		t.keys[id] = k
	}
	return k.value, k.err
}

func (t *frozenView) object(path naming.Path) frozenObject {
	o, ok := t.objects[path]
	if !ok {
		o.addrs, o.ok = t.View.Object(path)
		o.id, o.version = identity(t.View, path)
		t.objects[path] = o
	}
	return o
}

func (t *frozenView) Object(path naming.Path) (ObjectAddrs, bool) {
	o := t.object(path)
	return o.addrs, o.ok
}

// Identity implements Identifier, as the view it stands for does.
func (t *frozenView) Identity(path naming.Path) (string, time.Time) {
	o := t.object(path)
	return o.id, o.version
}

// AddrIdentity implements AddrIdentifier, as the view it stands for does:
// the object publishing an address is asked once, as its addresses are, and
// a view not telling the objects by address tells the one of the path.
func (t *frozenView) AddrIdentity(path naming.Path, addr netip.Addr) (string, time.Time) {
	if _, ok := t.View.(AddrIdentifier); !ok {
		return t.Identity(path)
	}
	id := frozenAddrID{path: path, addr: addr}
	i, ok := t.publishers[id]
	if !ok {
		i.ObjectID, i.Version = addrIdentity(t.View, path, addr)
		t.publishers[id] = i
	}
	return i.ObjectID, i.Version
}

func (t *frozenView) NamespaceObjects(namespace string) []naming.Path {
	l, ok := t.namespaces[namespace]
	if !ok {
		l = t.View.NamespaceObjects(namespace)
		t.namespaces[namespace] = l
	}
	return l
}

// objectsView is a view whose objects are the ones given.
type objectsView struct {
	View
	objects Objects
}

func (t objectsView) Object(path naming.Path) (ObjectAddrs, bool) {
	return t.objects.Object(path)
}

func (t objectsView) NamespaceObjects(namespace string) []naming.Path {
	return t.objects.NamespaceObjects(namespace)
}

// WithObjects returns the view with the objects given in place of its own:
// the view a driver reads from the configuration of its node knows none,
// and the daemon tells the ones the rules name.
func WithObjects(view View, objects Objects) View {
	return objectsView{View: view, objects: objects}
}

// addrIdentitiesView is a view whose objects are known as the ones given,
// each address by the object publishing it.
type addrIdentitiesView struct {
	View
	identities Identities
}

// Identity implements Identifier.
func (t addrIdentitiesView) Identity(path naming.Path) (string, time.Time) {
	i := t.identities[path.String()]
	return i.ObjectID, i.Version
}

// AddrIdentity implements AddrIdentifier: the object the daemon told of the
// address, none when it told the objects by address and none of this one,
// and the object of the path when it told that one alone.
func (t addrIdentitiesView) AddrIdentity(path naming.Path, addr netip.Addr) (string, time.Time) {
	i := t.identities[path.String()]
	if i.Publishers == nil {
		return i.ObjectID, i.Version
	}
	publisher := i.Publishers[addr.Unmap().WithZone("")]
	return publisher.ObjectID, publisher.Version
}

// WithAddrIdentities returns the view telling which objects publish the
// addresses of the paths, as the daemon told them with their addresses: the
// view a driver reads from the configuration of its node knows none, and the
// addresses a start records are then held by the objects that published
// them, not by their paths alone, see Holder.
//
// The object is the one of each address, as the daemon resolving the rules
// tells it, see AddrIdentifier: a start made while a node still tells its
// instance of an object deleted, beside the object created again under the
// path, would else record the address of the first as one of the second,
// which the record of the delete does not lift when that node is lost
// before the daemon resolves the rules of the start again.
func WithAddrIdentities(view View, identities Identities) View {
	return addrIdentitiesView{View: view, identities: identities}
}

// Object returns the addresses of an object, and false for one not held.
func (t Objects) Object(path naming.Path) (ObjectAddrs, bool) {
	addrs, ok := t[path.String()]
	return addrs, ok
}

// NamespaceObjects returns the objects held of a namespace, sorted.
func (t Objects) NamespaceObjects(namespace string) []naming.Path {
	l := make([]naming.Path, 0)
	for s := range t {
		if p, err := naming.ParsePath(s); err == nil && p.Namespace == namespace {
			l = append(l, p)
		}
	}
	slices.SortFunc(l, func(a, b naming.Path) int { return strings.Compare(a.String(), b.String()) })
	return l
}

// All returns the addresses of the resources, or of the one named, each
// once.
func (t ObjectAddrs) All(rid string) []netip.Addr {
	l := make([]netip.Addr, 0)
	add := func(addrs []netip.Addr) {
		for _, addr := range addrs {
			if addr.IsValid() && !slices.Contains(l, addr) {
				l = append(l, addr)
			}
		}
	}
	if rid != "" {
		add(t[rid])
		return l
	}
	rids := make([]string, 0, len(t))
	for s := range t {
		rids = append(rids, s)
	}
	slices.Sort(rids)
	for _, s := range rids {
		add(t[s])
	}
	return l
}

func (t Data) Object(path naming.Path) (ObjectAddrs, bool) {
	return t.Objects.Object(path)
}

func (t Data) NamespaceObjects(namespace string) []naming.Path {
	return t.Objects.NamespaceObjects(namespace)
}

// Key implements View.
func (t Data) Key(datastore naming.Path, key, namespace string) ([]byte, error) {
	keys, ok := t.Keys[datastore.String()]
	if !ok {
		return nil, ErrNoDatastore
	}
	if shares := t.Shares[datastore.String()]; datastore.Namespace != namespace && !slices.Contains(shares, "*") && !slices.Contains(shares, namespace) {
		return nil, ErrNotShared
	}
	value, ok := keys[key]
	if !ok {
		return nil, ErrNoKey
	}
	return []byte(value), nil
}

// IsUnsure implements Witness.
func (t Data) IsUnsure(holder Holder) bool {
	return slices.Contains(t.Unsure, holder.Path)
}

// Holders implements Witness.
func (t Data) Holders(addr netip.Addr) []naming.Path {
	l := make([]naming.Path, 0)
	for s, addrs := range t.Objects {
		if !slices.Contains(addrs.All(""), addr) {
			continue
		}
		if p, err := naming.ParsePath(s); err == nil {
			l = append(l, p)
		}
	}
	return l
}

func (t Data) Network(name string) ([]netip.Prefix, bool) {
	nw, ok := t.Networks[name]
	return nw.Ranges, ok
}

func (t Data) NetworkNode(name, node string) ([]netip.Prefix, bool) {
	nw, ok := t.Networks[name]
	if !ok {
		return nil, false
	}
	if _, ok := t.NodeAddrs[node]; !ok {
		return nil, false
	}
	return nw.NodeRanges[node], true
}

func (t Data) Node(name string) ([]netip.Addr, bool) {
	l, ok := t.NodeAddrs[name]
	return l, ok
}

func (t Data) Nodes() []string {
	l := make([]string, 0, len(t.NodeAddrs))
	for name := range t.NodeAddrs {
		l = append(l, name)
	}
	return l
}

func (t Data) DNS() []netip.Addr {
	return t.Nameservers
}
