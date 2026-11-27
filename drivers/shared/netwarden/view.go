package netwarden

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	// nodeView is the view of the cluster read from the configuration of
	// the node: its networks, the nodes it names, and the nameservers of
	// its resolver.
	//
	// It is what a driver resolves the rules over when it starts a
	// resource, and what the validation of a configuration looks the
	// networks and nodes up in.
	nodeView struct {
		node     *object.Node
		networks []network.Networker
		nodes    []string
		addrs    map[string][]netip.Addr
	}

	// subnetter is implemented by the networks whose configuration holds
	// the part each node hands out.
	subnetter interface {
		GetSubnetAs(nodename string) string
	}
)

// resolvConf is the file the nameservers of the node are read from,
// replaced by the tests.
var resolvConf = "/etc/resolv.conf"

// lookupTimeout bounds the resolution of the name of a node.
var lookupTimeout = 2 * time.Second

// objectsTimeout bounds the wait for the daemon to tell the addresses of
// the objects: a start does not hang on a daemon that does not answer.
var objectsTimeout = 5 * time.Second

func init() {
	netrule.RegisterLocalView(newNodeView)
	netrule.RegisterObjects(daemonObjects)
}

// daemonObjects asks the daemon of the node the addresses of the objects
// named and of the objects of the namespaces named, which it knows from the
// status of their instances on every node, and which objects they are,
// which it knows from their configurations.
func daemonObjects(objects []naming.Path, namespaces []string) (netrule.Objects, netrule.Identities, error) {
	c, err := client.New()
	if err != nil {
		return nil, nil, err
	}
	params := api.GetNetworkAddressParams{}
	if len(objects) > 0 {
		l := make([]string, len(objects))
		for i, p := range objects {
			l[i] = p.String()
		}
		params.Object = &l
	}
	if len(namespaces) > 0 {
		params.Namespace = &namespaces
	}
	ctx, cancel := context.WithTimeout(context.Background(), objectsTimeout)
	defer cancel()
	resp, err := c.GetNetworkAddressWithResponse(ctx, &params)
	if err != nil {
		return nil, nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil, fmt.Errorf("get the addresses of the objects: %s", resp.Status())
	}
	return ObjectsOf(resp.JSON200.Items), IdentitiesOf(resp.JSON200.Items), nil
}

// ObjectsOf returns the addresses of the objects the api answered with.
func ObjectsOf(items api.NetworkAddressItems) netrule.Objects {
	data := make(netrule.Objects, len(items))
	for _, item := range items {
		addrs := make(netrule.ObjectAddrs, len(item.Resources))
		for rid, l := range item.Resources {
			addrs[rid] = ParseNameservers(l)
		}
		data[item.Path] = addrs
	}
	return data
}

// IdentitiesOf returns which objects the api answered with the addresses
// of: their id and the version of their configuration, and the ones of the
// object publishing each address, where the daemon tells them.
//
// A daemon telling the object of a path and not the ones of its addresses,
// as one not upgraded yet, has every address of the path recorded with that
// object, as before. One knowing no identity at all tells none: the
// addresses are then recorded with the path of their object alone, and the
// daemon gives them the object it knows under the path when it resolves the
// rules again.
func IdentitiesOf(items api.NetworkAddressItems) netrule.Identities {
	identities := make(netrule.Identities, len(items))
	for _, item := range items {
		var identity netrule.Identity
		if item.ObjectId != nil {
			identity.ObjectID = *item.ObjectId
		}
		if item.Version != nil {
			identity.Version = *item.Version
		}
		if item.Holders != nil {
			// Never nil: a daemon telling the objects by address and
			// none of an address knows no configuration of the
			// instances publishing it, which is not the address
			// being the one of the object of the path.
			identity.Publishers = make(map[netip.Addr]netrule.Publisher)
			for _, holder := range *item.Holders {
				var publisher netrule.Publisher
				if holder.ObjectId != nil {
					publisher.ObjectID = *holder.ObjectId
				}
				if holder.Version != nil {
					publisher.Version = *holder.Version
				}
				for _, addr := range ParseNameservers(holder.Addresses) {
					identity.Publishers[addr] = publisher
				}
			}
		}
		if identity.ObjectID != "" || !identity.Version.IsZero() || identity.Publishers != nil {
			identities[item.Path] = identity
		}
	}
	return identities
}

func newNodeView() (netrule.View, error) {
	node, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	nodes, err := node.Nodes()
	if err != nil {
		return nil, err
	}
	return &nodeView{
		node:     node,
		networks: network.Networks(node),
		nodes:    nodes,
		addrs:    make(map[string][]netip.Addr),
	}, nil
}

func (t *nodeView) network(name string) network.Networker {
	for _, nw := range t.networks {
		if nw.Name() == name {
			return nw
		}
	}
	return nil
}

// Network implements netrule.View. A network of no range, as a l2_bridge
// one, is a network with no address.
func (t *nodeView) Network(name string) ([]netip.Prefix, bool) {
	nw := t.network(name)
	if nw == nil {
		return nil, false
	}
	return parseRange(nw.Network()), true
}

// NetworkNode implements netrule.View. The part of a node is the one the
// configuration holds: none is allocated here, which the setup of the
// network does.
func (t *nodeView) NetworkNode(name, node string) ([]netip.Prefix, bool) {
	nw := t.network(name)
	if nw == nil || !slices.Contains(t.nodes, node) {
		return nil, false
	}
	i, ok := nw.(subnetter)
	if !ok {
		return nil, true
	}
	return parseRange(i.GetSubnetAs(node)), true
}

// Node implements netrule.View: the address the listener of the node is
// configured on, and the ones its name resolves to.
func (t *nodeView) Node(name string) ([]netip.Addr, bool) {
	if !slices.Contains(t.nodes, name) {
		return nil, false
	}
	if l, ok := t.addrs[name]; ok {
		return l, true
	}
	l := make([]netip.Addr, 0)
	add := func(addr netip.Addr) {
		addr = addr.Unmap().WithZone("")
		if addr.IsValid() && !addr.IsUnspecified() && !slices.Contains(l, addr) {
			l = append(l, addr)
		}
	}
	if s := t.node.MergedConfig().GetStringAs(key.New("listener", "addr"), name); s != "" {
		if addr, err := netip.ParseAddr(s); err == nil {
			add(addr)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), lookupTimeout)
	defer cancel()
	if addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", name); err == nil {
		for _, addr := range addrs {
			add(addr)
		}
	}
	t.addrs[name] = l
	return l, true
}

// Nodes implements netrule.View.
func (t *nodeView) Nodes() []string {
	return t.nodes
}

// Object implements netrule.View. The configuration of the node says
// nothing of the objects: the daemon tells them, see netrule.WithObjects.
func (t *nodeView) Object(naming.Path) (netrule.ObjectAddrs, bool) {
	return nil, false
}

// NamespaceObjects implements netrule.View, as Object.
func (t *nodeView) NamespaceObjects(string) []naming.Path {
	return nil
}

// Key implements netrule.View: the key of a datastore the node holds the
// configuration of, decoded.
//
// It is read by root, the om command starting a resource or the daemon,
// from the file of the node: the key of a sec is decoded with the secret of
// the cluster, as the keys a resource installs are. Who is shown what it
// holds is the caller's business, see netrule.Rule.Hidden.
//
// Root reads every datastore, so the rule of whose keys an object may read
// is held here: the ones of its namespace, and the ones of the datastores
// of another namespace sharing their keys with it.
func (t *nodeView) Key(datastore naming.Path, name, namespace string) ([]byte, error) {
	return readKey(datastore, name, namespace)
}

// readKey reads a key of a datastore of the node for an object of the
// namespace given, and is replaced by the tests.
//
// The sharing is read with the key, from the same configuration, at every
// read: a datastore whose share keyword no longer admits the namespace is
// no longer read, whatever was read of it before, and nothing is said of
// the keys it holds.
var readKey = func(datastore naming.Path, name, namespace string) ([]byte, error) {
	if !datastore.Exists() {
		return nil, netrule.ErrNoDatastore
	}
	ds, err := object.NewDataStore(datastore, object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	if !ds.Allow(namespace) {
		// Its own namespace is always admitted.
		return nil, netrule.ErrNotShared
	}
	if !ds.HasKey(name) {
		return nil, netrule.ErrNoKey
	}
	return ds.DecodeKey(name)
}

// DNS implements netrule.View: the nameservers of the resolver of the node,
// which a command run on the node resolves through.
//
// The nameservers of cluster.dns are not added: where the resolver of the
// node is a stub forwarding to them, as the 127.0.0.53 of systemd-resolved,
// the command never talks to them, and naming them would open the nodes
// serving the cluster dns to it, on every port. A node whose resolv.conf
// names them has them here.
func (t *nodeView) DNS() []netip.Addr {
	return NodeNameservers()
}

// NodeNameservers returns the nameservers the resolv.conf of the node names,
// whatever the file is a link to: what a process of the host network
// namespace, or a container left with the resolver of the host, resolves
// through.
func NodeNameservers() []netip.Addr {
	return readNameservers(resolvConf)
}

// ParseNameservers returns the addresses of the nameservers named, as a
// driver writing a resolver of its own knows them, each once.
func ParseNameservers(l []string) []netip.Addr {
	addrs := make([]netip.Addr, 0, len(l))
	for _, s := range l {
		addr, err := netip.ParseAddr(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		if addr = addr.Unmap().WithZone(""); !slices.Contains(addrs, addr) {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

// readNameservers returns the nameservers a resolv.conf file names.
func readNameservers(p string) []netip.Addr {
	l := make([]netip.Addr, 0)
	f, err := os.Open(p)
	if err != nil {
		return l
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		words := strings.Fields(scanner.Text())
		if len(words) < 2 || words[0] != "nameserver" {
			continue
		}
		if addr, err := netip.ParseAddr(words[1]); err == nil {
			l = append(l, addr.WithZone(""))
		}
	}
	return l
}

// parseRange returns the range s names, none when it names none.
func parseRange(s string) []netip.Prefix {
	prefix, err := netip.ParsePrefix(s)
	if err != nil {
		return nil
	}
	return []netip.Prefix{prefix.Masked()}
}
