package netwarden

import (
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/hostname"
)

// view is the view the daemon resolves the rules over: the networks and the
// nodes as the configuration of the node has them, and the objects as the
// daemon heard of their instances, of every node.
//
// It is a netrule.Witness: the daemon knows which nodes it does not hear of,
// so which instances it can say nothing of. And a netrule.Identifier and a
// netrule.AddrIdentifier: it hears of the configuration and of the status
// of every instance, so of which object a path names, and of which object
// publishes each address.
type view struct {
	netrule.View

	// heard is what the daemon heard of the instances.
	heard heard

	// ghosts is the memory of the instances whose status went with no
	// word of what became of them, nil for a view with none.
	ghosts ghosts

	// away says a node does not tell its instances.
	away func(nodename string) bool

	// tombstones is the records of the deletes heard of, nil for a view
	// hearing none.
	tombstones tombstones

	// waited is told an instance newly remembered as one whose node
	// does not tell it, nil when nobody is.
	waited func(p naming.Path, nodename string)
}

// heard is what the daemon heard of the instances of the objects, by object
// then by node: their status and their configuration, as the events said
// them, in the order they were published.
//
// The rules are not resolved over the instance data of the daemon, which is
// written ahead of the events, and rewritten instance by instance when a
// node sends the whole of its data: read in between, an instance that never
// changed is found missing, and its addresses were taken out of the filters
// for the time of a few events.
type heard map[naming.Path]map[string]*heardInstance

// heardInstance is what the daemon heard of an instance.
type heardInstance struct {
	// hasStatus says the instance has a status, addrs being the
	// addresses its resources publish in it.
	hasStatus bool
	addrs     netrule.ObjectAddrs

	// hasConfig says the instance has a configuration, scope being the
	// nodes of the object as it has them, id the id of the object, empty
	// when the configuration tells none, and version the modification
	// time of the configuration, as the node committing it set it.
	hasConfig bool
	scope     []string
	id        string
	version   time.Time
}

func (t heard) instance(p naming.Path, nodename string) *heardInstance {
	if t[p] == nil {
		t[p] = make(map[string]*heardInstance)
	}
	if t[p][nodename] == nil {
		t[p][nodename] = &heardInstance{}
	}
	return t[p][nodename]
}

// setStatus records the status of an instance, and says whether the
// addresses its resources publish changed: a status heard for the first
// time changes them, and one whose other fields only moved, as a status
// evaluated again with nothing changed, does not.
func (t heard) setStatus(p naming.Path, nodename string, st *instance.Status) bool {
	i := t.instance(p, nodename)
	addrs := objectAddrs(map[string]*instance.Status{nodename: st})
	changed := !i.hasStatus || !sameObjectAddrs(i.addrs, addrs)
	i.hasStatus, i.addrs = true, addrs
	return changed
}

// sameObjectAddrs says two sets of addresses of the resources of an object
// are the same: the same resources, publishing the same addresses.
func sameObjectAddrs(a, b netrule.ObjectAddrs) bool {
	if len(a) != len(b) {
		return false
	}
	for rid, l := range a {
		m, ok := b[rid]
		if !ok || !slices.Equal(l, m) {
			return false
		}
	}
	return true
}

func (t heard) setConfig(p naming.Path, nodename string, scope []string, id string, version time.Time) {
	i := t.instance(p, nodename)
	i.hasConfig, i.scope, i.id, i.version = true, append([]string{}, scope...), id, version
}

// identity returns which object the path names: the id and the version of
// the newest configuration heard of its instances, each zero when none
// tells it.
//
// It is the object an address remembered with no holder is given, as one
// an om knowing no holder recorded, see netrule.Held.Keep: nothing says it
// was another. It is not the object of an address an instance publishes,
// which addrIdentity tells.
func (t heard) identity(p naming.Path) (id string, version time.Time) {
	for _, i := range t[p] {
		if !i.hasConfig {
			continue
		}
		if version.IsZero() || i.version.After(version) {
			id, version = i.id, i.version
		}
	}
	return id, version
}

// addrIdentity returns which object publishes the address under the path:
// the id and the version of the newest configuration heard of the instances
// whose status holds the address, each zero when none tells it.
//
// The path may be known by a newer configuration than the one of the
// instance publishing the address: the object was deleted and created
// again, or restored, and a node still tells its instance of the object
// deleted. The address is the one of that object, and the record of its
// delete lifts it, as the node is lost before it says its instance is gone.
// Given the newest configuration of the path, the address was kept allowed
// until the node told the path again.
//
// An address several instances publish is the one of the newest of their
// configurations, the one the record of a delete matches less of: a node
// holding the older one is about to fetch the other.
//
// The configuration of a publishing instance may not be known: its status
// is heard before, and the data of a node lost is dropped configuration
// first. No object is told then, not the one of the path, so the holder the
// address was remembered with is kept, see netrule.Held.remember.
func (t heard) addrIdentity(p naming.Path, addr netip.Addr) (id string, version time.Time) {
	for _, i := range t[p] {
		if !i.hasConfig || !i.hasStatus || !slices.Contains(i.addrs.All(""), addr) {
			continue
		}
		if version.IsZero() || i.version.After(version) {
			id, version = i.id, i.version
		}
	}
	return id, version
}

func (t heard) dropStatus(p naming.Path, nodename string) {
	if i := t[p][nodename]; i != nil {
		i.hasStatus, i.addrs = false, nil
		t.prune(p, nodename)
	}
}

func (t heard) dropConfig(p naming.Path, nodename string) {
	if i := t[p][nodename]; i != nil {
		i.hasConfig, i.scope, i.id, i.version = false, nil, "", time.Time{}
		t.prune(p, nodename)
	}
}

// prune forgets an instance with no status and no configuration, and an
// object with no instance.
func (t heard) prune(p naming.Path, nodename string) {
	if i := t[p][nodename]; i != nil && !i.hasStatus && !i.hasConfig {
		delete(t[p], nodename)
	}
	if len(t[p]) == 0 {
		delete(t, p)
	}
}

// ghosts is the instances, by node, whose status the daemon lost with no
// word of what became of them: their node went away, or was not heard of
// when they were looked for. What they hold is not known, which is not
// known gone, until something says so, see Manager.onEvent.
type ghosts map[string]map[naming.Path]bool

func (t ghosts) has(p naming.Path, nodename string) bool {
	return t[nodename][p]
}

// add remembers an instance, and says it was not remembered already.
func (t ghosts) add(p naming.Path, nodename string) bool {
	if t[nodename] == nil {
		t[nodename] = make(map[naming.Path]bool)
	}
	if t[nodename][p] {
		return false
	}
	t[nodename][p] = true
	return true
}

// drop forgets an instance, and says it was remembered.
func (t ghosts) drop(p naming.Path, nodename string) bool {
	if !t[nodename][p] {
		return false
	}
	delete(t[nodename], p)
	return true
}

// keepNodes forgets the instances of an object on the nodes it does not run
// on, scope being the nodes it does, when it names some, and returns how
// many it forgot.
func (t ghosts) keepNodes(p naming.Path, scope []string) int {
	if len(scope) == 0 {
		return 0
	}
	n := 0
	for nodename := range t {
		if !slices.Contains(scope, nodename) && t.drop(p, nodename) {
			n++
		}
	}
	return n
}

// tombstones is the records of the delete of the objects the daemon heard
// of, by object then by the id of the orchestration that deleted it.
//
// A record is published by every node holding it, and again by a node
// starting, long after the object was created again under its path: it is
// retained for a day. So a record says nothing of a path by itself. It is
// weighed against each address a filter remembers, by the object that
// published the address, see netrule.Holder.IsDeletedBy: the record of the
// delete of that object takes the address away, and any other is none of
// it. It is forgotten as a node removes it.
//
// What is compared is the id of the object, and the version of its
// configuration with the one deleted, both set by the node that committed
// them: no time of a node is compared with one of another.
type tombstones map[naming.Path]map[string]tombstone.Record

// add remembers a record, and says it was not remembered already.
func (t tombstones) add(p naming.Path, id string, rec tombstone.Record) bool {
	if _, ok := t[p][id]; ok {
		return false
	}
	if t[p] == nil {
		t[p] = make(map[string]tombstone.Record)
	}
	t[p][id] = rec
	return true
}

// drop forgets a record.
func (t tombstones) drop(p naming.Path, id string) {
	delete(t[p], id)
	if len(t[p]) == 0 {
		delete(t, p)
	}
}

// weigh says what the records of the path tell of the object holding an
// address: deleted, one being the record of its delete, or blind, the
// holder telling no version to judge by, and a record being the delete of
// an object of its path it may be.
func (t tombstones) weigh(p naming.Path, holder netrule.Holder) (deleted, blind bool) {
	for _, rec := range t[p] {
		switch {
		case holder.IsDeletedBy(rec):
			deleted = true
		case holder.Version.IsZero() && (holder.ObjectID == "" || rec.ObjectID == "" || holder.ObjectID == rec.ObjectID):
			blind = true
		}
	}
	return deleted, blind
}

// hasConfigFile says the node holds the configuration of the object, and is
// replaced by the tests.
var hasConfigFile = func(p naming.Path) bool {
	return file.Exists(p.ConfigFile())
}

// parseAddr reads an address a resource publishes, with or without the
// length of its network.
func parseAddr(s string) (netip.Addr, bool) {
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr.Unmap().WithZone(""), true
	}
	if prefix, err := netip.ParsePrefix(s); err == nil {
		return prefix.Addr().Unmap(), true
	}
	return netip.Addr{}, false
}

// objectAddrs returns the addresses the resources of an object publish, from
// the status of its instances, by node: every resource is there, the ones
// publishing no address with none, and an address the instances of several
// nodes publish is there once.
func objectAddrs(statuses map[string]*instance.Status) netrule.ObjectAddrs {
	addrs := make(netrule.ObjectAddrs)
	nodes := make([]string, 0, len(statuses))
	for nodename := range statuses {
		nodes = append(nodes, nodename)
	}
	slices.Sort(nodes)
	for _, nodename := range nodes {
		st := statuses[nodename]
		if st == nil {
			continue
		}
		for rid, rstat := range st.Resources {
			l, ok := addrs[rid]
			if !ok {
				l = make([]netip.Addr, 0)
			}
			for _, s := range rstat.IPAddrs() {
				if addr, ok := parseAddr(s); ok && !slices.Contains(l, addr) {
					l = append(l, addr)
				}
			}
			addrs[rid] = l
		}
	}
	return addrs
}

// clusterNodes returns the nodes of the cluster, this one among them.
func clusterNodes() []string {
	var nodes []string
	if cfg := cluster.ConfigData.Get(); cfg != nil {
		nodes = append(nodes, cfg.Nodes...)
	}
	if localhost := hostname.Hostname(); !slices.Contains(nodes, localhost) {
		nodes = append(nodes, localhost)
	}
	return nodes
}

// hasObject says the cluster has the object: a node holds a configuration
// or a status of an instance of it.
func hasObject(p naming.Path) bool {
	return len(instance.ConfigData.GetNodesByPath(p)) > 0 || len(instance.StatusData.GetNodesByPath(p)) > 0
}

// namespaceObjects returns the objects of a namespace the cluster has,
// sorted.
func namespaceObjects(namespace string) []naming.Path {
	seen := make(map[naming.Path]bool)
	l := make([]naming.Path, 0)
	add := func(paths []naming.Path) {
		for _, p := range paths {
			if p.Namespace != namespace || seen[p] {
				continue
			}
			seen[p] = true
			l = append(l, p)
		}
	}
	for _, nodename := range clusterNodes() {
		add(instance.ConfigData.GetPathsByNode(nodename))
		add(instance.StatusData.GetPathsByNode(nodename))
	}
	slices.SortFunc(l, func(a, b naming.Path) int { return strings.Compare(a.String(), b.String()) })
	return l
}

// Objects returns the addresses of the objects named and of the objects of
// the namespaces named, as the data of the daemon has them, an object the
// cluster does not have left out.
//
// It is what the api hands to a driver starting a resource whose rules name
// objects, and what the validation of a configuration looks them up in:
// read once, from the instance data, where the daemon keeping the filters
// current follows the events.
func Objects(objects []naming.Path, namespaces []string) netrule.Objects {
	data := make(netrule.Objects)
	add := func(p naming.Path) {
		if _, ok := data[p.String()]; ok || !hasObject(p) {
			return
		}
		data[p.String()] = objectAddrs(instance.StatusData.GetByPath(p))
	}
	for _, p := range objects {
		add(p)
	}
	for _, namespace := range namespaces {
		for _, p := range namespaceObjects(namespace) {
			add(p)
		}
	}
	return data
}

// Identity returns which object the path names, as the data of the daemon
// has the configuration of its instances: what the view of the daemon
// following the events tells, see heard.identity, read once.
func Identity(p naming.Path) (id string, version time.Time) {
	return heardOf(p).identity(p)
}

// heardOf returns what the data of the daemon has of the instances of an
// object, as the daemon following the events heard it: the view the api
// tells the objects of the addresses from is the one the filters are kept
// current over, read once.
func heardOf(p naming.Path) heard {
	h := make(heard)
	for nodename, cfg := range instance.ConfigData.GetByPath(p) {
		if cfg != nil {
			h.setConfig(p, nodename, cfg.Scope, cfg.ID, cfg.UpdatedAt)
		}
	}
	for nodename, st := range instance.StatusData.GetByPath(p) {
		if st != nil {
			h.setStatus(p, nodename, st)
		}
	}
	return h
}

// Identities returns which objects publish the addresses of the paths of
// the objects given, each address with its own, see heard.addrIdentity: an
// address no configuration of the instances publishing it is known of is
// told no object.
//
// It is what the api hands to a driver starting a resource with the
// addresses of the objects, for the start to record each address with the
// object publishing it: a record of the delete of an object created again
// since would else lift the addresses of a holder known by its path alone,
// until the daemon resolves the rules again.
//
// The object is told for each address, as the daemon resolving the rules
// tells it, and by the same code: a path names two objects while a node
// back from a delete still tells its instance of the object deleted, beside
// the object created again. Told once for the path, the address of the
// first was recorded as one of the second, and kept allowed while that node
// was lost before the daemon found the record of the start.
//
// The object of the path is told too, for an om knowing no object by
// address: the newest of the ones publishing an address, and the one the
// path names, see heard.identity, when no address is published.
func Identities(objects netrule.Objects) netrule.Identities {
	identities := make(netrule.Identities)
	for s, addrs := range objects {
		p, err := naming.ParsePath(s)
		if err != nil {
			continue
		}
		h := heardOf(p)
		identity := netrule.Identity{Publishers: make(map[netip.Addr]netrule.Publisher)}
		published := addrs.All("")
		for _, addr := range published {
			id, version := h.addrIdentity(p, addr)
			if id == "" && version.IsZero() {
				continue
			}
			identity.Publishers[addr] = netrule.Publisher{ObjectID: id, Version: version}
			if identity.Version.IsZero() || version.After(identity.Version) {
				identity.ObjectID, identity.Version = id, version
			}
		}
		if len(published) == 0 {
			identity.ObjectID, identity.Version = h.identity(p)
		}
		identities[s] = identity
	}
	return identities
}

// Object implements netrule.View: the addresses the instances of the
// object publish, all nodes together.
func (t view) Object(p naming.Path) (netrule.ObjectAddrs, bool) {
	instances := t.heard[p]
	if len(instances) == 0 {
		return nil, false
	}
	nodes := make([]string, 0, len(instances))
	for nodename := range instances {
		nodes = append(nodes, nodename)
	}
	slices.Sort(nodes)
	addrs := make(netrule.ObjectAddrs)
	for _, nodename := range nodes {
		for rid, l := range instances[nodename].addrs {
			merged, ok := addrs[rid]
			if !ok {
				merged = make([]netip.Addr, 0, len(l))
			}
			for _, addr := range l {
				if !slices.Contains(merged, addr) {
					merged = append(merged, addr)
				}
			}
			addrs[rid] = merged
		}
	}
	return addrs, true
}

// NamespaceObjects implements netrule.View.
func (t view) NamespaceObjects(namespace string) []naming.Path {
	l := make([]naming.Path, 0)
	for p := range t.heard {
		if p.Namespace == namespace {
			l = append(l, p)
		}
	}
	slices.SortFunc(l, func(a, b naming.Path) int { return strings.Compare(a.String(), b.String()) })
	return l
}

// isNodeUnsure says the node does not tell its instances for now: the
// daemon holds no data of it, lost or not heard of yet, or its daemon is
// away, starting, rejoining the cluster or shutting down, the instances it
// tells then being the ones it has discovered so far.
//
// The data of a lost peer is dropped node monitor first, so an instance
// status found gone with its node monitor is one of a node lost, not of an
// instance deleted. And a daemon restarting tells its peers a node with no
// instance, then its instances as it finds them: what it runs never
// stopped.
func isNodeUnsure(nodename string) bool {
	monitor := node.MonitorData.GetByNode(nodename)
	return monitor == nil || monitor.State.IsAwayFromObjects()
}

// IsUnsure implements netrule.Witness: nothing tells the status of an
// instance of the object holding an address, on a node the object may run
// on.
//
// The nodes are the ones the configuration of the object names, as an
// instance known has it, and every node of the cluster when none is known:
// the object may be one of a node not heard of.
//
// The status of an instance the daemon does not hold is not told while the
// daemon knows the configuration of the instance and waits for its status.
// On this node, it is not told while the node holds a configuration of the
// object the daemon has not read yet, as it starts. On a peer, it is not
// told when the peer does not tell its instances, and stays so once it
// does, until it tells this one: a daemon back in the cluster tells its
// instances one after the other, over seconds.
//
// A record of the delete of the object holding the address is the word of
// the instances no node tells: they are gone, or deleted as their node is
// back, which then says nothing of them. So the instance of a peer not
// telling its instances is not waited for, which would keep the address for
// good: an allowed address goes on the evidence it is gone, and a denied
// one when its object is deleted, which the record is. The record is the
// one of that holder, see tombstones: the one of the object its path named
// before is none of it, and its instance is waited for as any other.
//
// A holder telling no version, as one an om knowing no holder recorded, is
// not judged by a record, so its instance is waited for while its node is
// not heard of. The node back, and telling nothing of it since, a record of
// its path is taken for its delete: nothing else would ever say the object
// is gone, a node saying nothing of what it does not hold.
func (t view) IsUnsure(holder netrule.Holder) bool {
	p, err := naming.ParsePath(holder.Path)
	if err != nil {
		return false
	}
	localhost := hostname.Hostname()
	instances := t.heard[p]
	var scope []string
	for _, i := range instances {
		if len(i.scope) > 0 {
			scope = i.scope
			break
		}
	}
	deleted, blind := t.tombstones.weigh(p, holder)
	unsure := false
	for _, nodename := range clusterNodes() {
		if scope != nil && !slices.Contains(scope, nodename) {
			continue
		}
		i := instances[nodename]
		if i != nil && i.hasStatus {
			continue
		}
		if i != nil && i.hasConfig {
			unsure = true
			continue
		}
		if nodename == localhost {
			// This node tells its instances from the configurations
			// it holds, whatever its daemon is doing.
			unsure = unsure || hasConfigFile(p)
			continue
		}
		if deleted {
			continue
		}
		if t.isAway(nodename) {
			// Remembered for the day the node is back: it will not
			// have told this instance yet.
			if t.ghosts != nil && t.ghosts.add(p, nodename) && t.waited != nil {
				t.waited(p, nodename)
			}
			unsure = true
			continue
		}
		if t.ghosts.has(p, nodename) && !blind {
			unsure = true
		}
	}
	return unsure
}

// Identity implements netrule.Identifier.
func (t view) Identity(p naming.Path) (string, time.Time) {
	return t.heard.identity(p)
}

// AddrIdentity implements netrule.AddrIdentifier.
func (t view) AddrIdentity(p naming.Path, addr netip.Addr) (string, time.Time) {
	return t.heard.addrIdentity(p, addr)
}

func (t view) isAway(nodename string) bool {
	if t.away != nil {
		return t.away(nodename)
	}
	return isNodeUnsure(nodename)
}

// Holders implements netrule.Witness.
func (t view) Holders(addr netip.Addr) []naming.Path {
	l := make([]naming.Path, 0)
	for p, instances := range t.heard {
		for _, i := range instances {
			if slices.Contains(i.addrs.All(""), addr) {
				l = append(l, p)
				break
			}
		}
	}
	return l
}
