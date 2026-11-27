package netwarden

import (
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

// fakeSlice is a slice holding the filter it is set, as systemd does.
type fakeSlice struct {
	allow, deny []string
	sets        int
	hasFilter   bool
	members     bool
	procs       bool
	setErr      error
}

func (t *fakeSlice) HasIPFilter() bool { return t.hasFilter }
func (t *fakeSlice) HasMembers() bool  { return t.members }
func (t *fakeSlice) HasProcs() bool    { return t.procs }

func (t *fakeSlice) SetIPFilter(allow, deny []string) error {
	if t.setErr != nil {
		return t.setErr
	}
	t.sets++
	t.hasFilter = true
	t.allow, t.deny = allow, deny
	return nil
}

func (t *fakeSlice) allowed() string { return strings.Join(t.allow, " ") }
func (t *fakeSlice) denied() string  { return strings.Join(t.deny, " ") }

// world is a cluster of the nodes n1, this node, and n2, with the data the
// daemon holds of it, and the slices of the resources of n1.
type world struct {
	t      *testing.T
	m      *Manager
	dir    string
	slices map[string]*fakeSlice
	files  map[naming.Path][]string

	// local is the objects this node holds the configuration of.
	local map[naming.Path]bool

	// conf is the rules the configurations of the objects of this node
	// give their resources, by resource.
	conf map[naming.Path]map[string]rules
}

const implicit = "10.29.0.53/32 127.0.0.0/8 ::1/128"

func path(t *testing.T, s string) naming.Path {
	t.Helper()
	p, err := naming.ParsePath(s)
	require.NoError(t, err)
	return p
}

func newWorld(t *testing.T) *world {
	t.Helper()
	hostname.SetHostnameForGoTest("n1")
	instance.InitData()
	node.InitData()
	cluster.InitData()
	tombstone.InitData()
	cluster.ConfigData.Set(&cluster.Config{Nodes: []string{"n1", "n2"}})
	w := &world{
		t:      t,
		dir:    t.TempDir(),
		slices: make(map[string]*fakeSlice),
		files:  make(map[naming.Path][]string),
		local:  make(map[naming.Path]bool),
		conf:   make(map[naming.Path]map[string]rules),
	}
	savedView, savedSliceOf, savedFiles, savedPaths, savedHasConfig := localView, sliceOf, recordFiles, installedPaths, hasConfigFile
	savedConfigured, savedNamesRules := configured, namesRules
	t.Cleanup(func() {
		localView, sliceOf, recordFiles, installedPaths, hasConfigFile = savedView, savedSliceOf, savedFiles, savedPaths, savedHasConfig
		configured, namesRules = savedConfigured, savedNamesRules
		hostname.SetHostnameForGoTest("")
	})
	configured = func(p naming.Path) (map[string]rules, error) { return w.conf[p], nil }
	namesRules = func(p naming.Path) bool { return len(w.conf[p]) > 0 }
	hasConfigFile = func(p naming.Path) bool { return w.local[p] }
	localView = func() (netrule.View, error) {
		return netrule.Data{
			Networks:    map[string]netrule.DataNetwork{"lan": {Ranges: []netip.Prefix{netip.MustParsePrefix("192.168.0.0/24")}}},
			NodeAddrs:   map[string][]netip.Addr{"n1": {netip.MustParseAddr("10.29.0.11")}, "n2": {netip.MustParseAddr("10.29.0.12")}},
			Nameservers: []netip.Addr{netip.MustParseAddr("10.29.0.53")},
		}, nil
	}
	sliceOf = func(id string) slice { return w.slice(id) }
	recordFiles = func(p naming.Path) []string {
		l := make([]string, 0)
		for _, file := range w.files[p] {
			if _, err := os.Stat(file); err == nil {
				l = append(l, file)
			}
		}
		return l
	}
	installedPaths = func() (naming.Paths, error) {
		l := make(naming.Paths, 0)
		for p := range w.files {
			l = append(l, p)
		}
		for p := range w.conf {
			if _, ok := w.files[p]; !ok {
				l = append(l, p)
			}
		}
		return l, nil
	}
	w.m = NewManager(nil)
	w.m.log = plog.NewDefaultLogger()
	w.known("n1")
	w.known("n2")
	return w
}

func (w *world) slice(id string) *fakeSlice {
	s, ok := w.slices[id]
	if !ok {
		s = &fakeSlice{}
		w.slices[id] = s
	}
	return s
}

// known has the daemon hear of a node, its daemon in the cluster.
func (w *world) known(nodename string) {
	w.state(nodename, node.MonitorStateIdle)
}

// state has the daemon hear of the node monitor state of a node.
func (w *world) state(nodename string, state node.MonitorState) {
	node.MonitorData.Set(nodename, &node.Monitor{State: state})
	w.m.onEvent(&msgbus.NodeMonitorUpdated{Node: nodename, Value: node.Monitor{State: state}})
}

// back has the daemon hear again of a peer, whose daemon starts: it sends
// the whole of its data, with no instance yet, rejoins the cluster, and is
// in it before it has told its instances. The function returned has it tell
// the instances given, each "<path>" or "<path>=<address>".
func (w *world) back(nodename string, instances ...string) func() {
	w.t.Helper()
	w.m.onEvent(&msgbus.NodeDataUpdated{Node: nodename, Value: node.Node{Monitor: node.Monitor{State: node.MonitorStateRejoin}}})
	w.state(nodename, node.MonitorStateRejoin)
	w.full(nodename)
	w.known(nodename)
	return func() {
		for _, s := range instances {
			name, addr, ok := strings.Cut(s, "=")
			if ok {
				w.status(name, nodename, addr)
			} else {
				w.status(name, nodename)
			}
		}
	}
}

// full has a peer in the cluster send the whole of its data, holding the
// instances given: the ones it found so far, when its daemon is just back.
func (w *world) full(nodename string, instances ...string) {
	data := node.Node{Monitor: node.Monitor{State: node.MonitorStateIdle}, Instance: make(map[string]instance.Instance)}
	for _, s := range instances {
		data.Instance[s] = instance.Instance{Config: &instance.Config{}, Status: &instance.Status{}}
	}
	w.m.onEvent(&msgbus.NodeDataUpdated{Node: nodename, Value: data})
}

// lost has the daemon lose a peer, as its data is dropped then: the node
// monitor first, then the instances.
func (w *world) lost(nodename string) {
	node.MonitorData.Unset(nodename)
	w.m.onEvent(&msgbus.NodeMonitorDeleted{Node: nodename})
	for p := range instance.ConfigData.GetByNode(nodename) {
		instance.ConfigData.Unset(p, nodename)
		w.m.onEvent(&msgbus.InstanceConfigDeleted{Path: p, Node: nodename})
	}
	for p := range instance.StatusData.GetByNode(nodename) {
		instance.StatusData.Unset(p, nodename)
		w.m.onEvent(&msgbus.InstanceStatusDeleted{Path: p, Node: nodename})
	}
}

// status sets the status of an instance: its ip#1 resource publishing the
// addresses given.
func (w *world) status(s, nodename string, addrs ...string) {
	p := path(w.t, s)
	info := map[string]any{}
	if len(addrs) > 0 {
		info[resource.IPAddrInfoKey] = addrs[0]
		info[resource.IPAddrsInfoKey] = addrs
	}
	st := instance.Status{Resources: instance.ResourceStatuses{
		"ip#1": resource.Status{Status: status.Up, Info: info},
		"fs#1": resource.Status{Status: status.Up},
	}}
	cfg := instance.Config{Scope: []string{"n1", "n2"}}
	if instance.ConfigData.GetByPathAndNode(p, nodename) == nil {
		instance.ConfigData.Set(p, nodename, &cfg)
		w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: p, Node: nodename, Value: cfg})
	}
	instance.StatusData.Set(p, nodename, &st)
	w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: p, Node: nodename, Value: st})
}

// view returns the view the daemon resolves over.
func (w *world) view() view {
	w.t.Helper()
	v, err := w.m.view()
	require.NoError(w.t, err)
	return v.(view)
}

// deleted deletes an instance.
func (w *world) deleted(s, nodename string) {
	p := path(w.t, s)
	instance.ConfigData.Unset(p, nodename)
	w.m.onEvent(&msgbus.InstanceConfigDeleted{Path: p, Node: nodename})
	instance.StatusData.Unset(p, nodename)
	w.m.onEvent(&msgbus.InstanceStatusDeleted{Path: p, Node: nodename})
}

// start writes the record of the start of the resource app#1 of an object of
// this node, as its driver does, its slice holding the filter and a unit,
// and returns the slice. The daemon hears of it by the status of the
// instance.
func (w *world) start(s, allow, deny string) *fakeSlice {
	w.t.Helper()
	fs := w.record(s, allow, deny)
	p := path(w.t, s)
	w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: p, Node: "n1"})
	return fs
}

func (w *world) record(s, allow, deny string) *fakeSlice {
	w.t.Helper()
	p := path(w.t, s)
	id := "/opensvc.slice/" + strings.ReplaceAll(s, "/", ".") + ".slice"
	rec := netrule.Record{NetRules: netrule.TextsOf(listOf(allow, deny)), Namespace: p.Namespace, PG: id}
	base, err := localView()
	require.NoError(w.t, err)
	// A start asks the daemon the objects, and has no memory.
	objects, namespaces := netrule.References(p.Namespace, mustParse(w.t, listOf(allow, deny)))
	rec, _, err = rec.Refresh(netrule.WithObjects(base, Objects(objects, namespaces)))
	require.NoError(w.t, err)
	file := filepath.Join(w.dir, strings.ReplaceAll(s, "/", "."), "app#1", recordName)
	require.NoError(w.t, rec.Save(file))
	w.files[p] = []string{file}
	fs := w.slice(id)
	fs.members = true
	flatAllow, flatDeny := rec.Filter.Flat()
	fs.allow, fs.deny = netrule.Strings(flatAllow), netrule.Strings(flatDeny)
	fs.hasFilter = !rec.Filter.Flattened().IsZero()
	fs.sets = 0
	return fs
}

func mustParse(t *testing.T, value string) netrule.Rules {
	t.Helper()
	rules, err := netrule.Parse(netrule.Keyword, value)
	require.NoError(t, err)
	return rules
}

// listOf returns the value of net_rules saying what the rules to allow and
// the rules to deny given say together, each a line holding a noun and what
// qualifies it, as most tests here tell what a filter lets through and what
// it does not: the deny rules first, so they win, then the allow rules,
// and a last allow any when there is no allow rule, the rest being let
// through then. A value holding verbs already is a list written whole, and
// returned as it is, the other being empty.
func listOf(allow, deny string) string {
	if strings.HasPrefix(strings.TrimSpace(allow), "allow ") || strings.HasPrefix(strings.TrimSpace(allow), "deny ") {
		return allow
	}
	lines := func(verb, value string) []string {
		l := make([]string, 0)
		for _, line := range strings.Split(value, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				l = append(l, verb+" "+line)
			}
		}
		return l
	}
	l := lines("deny", deny)
	allowed := lines("allow", allow)
	if len(l) > 0 && len(allowed) == 0 {
		allowed = []string{"allow any"}
	}
	return strings.Join(append(l, allowed...), "\n")
}

// flatAllow and flatDeny return the allowed and the denied list a slice is
// given for a filter.
func flatAllow(filter netrule.Filter) []string {
	allow, _ := filter.Flat()
	return netrule.Strings(allow)
}

func flatDeny(filter netrule.Filter) []string {
	_, deny := filter.Flat()
	return netrule.Strings(deny)
}

// implicitSteps is how a filter says what the implicit rules resolve to
// over the view of the world.
const implicitSteps = "implicit allow 127.0.0.0/8 ::1/128, implicit allow 10.29.0.53/32, implicit deny any"

// allowedOf and deniedOf return the addresses a memory holds of the allow
// rules, and of the deny rules, all of them together.
func allowedOf(memory netrule.Memory) netrule.Held { return sideOf(memory, "allow ") }
func deniedOf(memory netrule.Memory) netrule.Held  { return sideOf(memory, "deny ") }

func sideOf(memory netrule.Memory, verb string) netrule.Held {
	held := make(netrule.Held)
	for text, l := range memory {
		if !strings.HasPrefix(text, verb) {
			continue
		}
		for addr, holder := range l {
			held[addr] = holder
		}
	}
	return held
}

func (w *world) loaded(s string) netrule.Record {
	w.t.Helper()
	rec, err := netrule.LoadRecord(w.files[path(w.t, s)][0])
	require.NoError(w.t, err)
	return rec
}

func TestObjectAddrsIsTheUnionOverTheInstances(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n1", "10.0.1.5")
	w.status("test/svc/db", "n2", "10.0.1.5", "10.0.1.6/24", "not an address")
	w.status("prod/svc/db", "n2", "fd00::5")
	w.status("test/svc/down", "n2")

	data := Objects([]naming.Path{path(t, "test/svc/db"), path(t, "test/svc/nope")}, []string{"prod", "nowhere"})
	assert.Equal(t, netrule.Objects{
		"test/svc/db": {
			"ip#1": {netip.MustParseAddr("10.0.1.5"), netip.MustParseAddr("10.0.1.6")},
			"fs#1": {},
		},
		"prod/svc/db": {
			"ip#1": {netip.MustParseAddr("fd00::5")},
			"fs#1": {},
		},
	}, data, "an address is the one of the object whichever instance publishes it, and a resource with none is still one")

	v := w.view()
	_, ok := v.Object(path(t, "test/svc/nope"))
	assert.False(t, ok)
	addrs, ok := v.Object(path(t, "test/svc/down"))
	assert.True(t, ok, "an object with no address is an object")
	assert.Empty(t, addrs.All(""))
	assert.Equal(t, []naming.Path{path(t, "test/svc/db"), path(t, "test/svc/down")}, v.NamespaceObjects("test"))
	assert.Equal(t, []naming.Path{path(t, "test/svc/db")}, v.Holders(netip.MustParseAddr("10.0.1.6")))
}

// unsureView asks a view of the object of a path, known by no
// configuration.
type unsureView struct{ view }

func (t unsureView) IsUnsure(p naming.Path) bool {
	return t.view.IsUnsure(netrule.Holder{Path: p.String()})
}

func TestIsUnsure(t *testing.T) {
	w := newWorld(t)
	db := path(t, "test/svc/db")
	v := unsureView{w.view()}
	config := func(p naming.Path, nodename string, scope ...string) {
		w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: p, Node: nodename, Value: instance.Config{Scope: scope}})
	}
	assert.False(t, v.IsUnsure(db), "every node is in the cluster, none holds the object: it does not exist")
	w.status("test/svc/db", "n2", "10.0.1.5")
	assert.False(t, v.IsUnsure(db))

	// An instance discovered whose status is still to come.
	config(db, "n1", "n1", "n2")
	assert.True(t, v.IsUnsure(db))
	w.m.onEvent(&msgbus.InstanceConfigDeleted{Path: db, Node: "n1"})
	assert.False(t, v.IsUnsure(db))

	// The configuration this node holds, not read yet by a daemon
	// starting.
	w.local[db] = true
	assert.True(t, v.IsUnsure(db))
	w.local[db] = false
	assert.False(t, v.IsUnsure(db))

	// This node rejoining says nothing more: its configurations do.
	w.state("n1", node.MonitorStateRejoin)
	assert.False(t, v.IsUnsure(db))
	w.known("n1")

	// A peer not heard of the object does not run on.
	web := path(t, "test/svc/web")
	config(web, "n1", "n1")
	w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: web, Node: "n1"})
	w.lost("n2")
	assert.False(t, v.IsUnsure(web))
	assert.True(t, v.IsUnsure(path(t, "test/svc/unknown")), "an object no instance of is known may be one of the peer")

	// A peer of the object not heard of, or whose daemon is away, as
	// one rejoining, which tells the instances it has found so far.
	config(web, "n1", "n1", "n2")
	assert.True(t, v.IsUnsure(web))
	for _, state := range []node.MonitorState{node.MonitorStateInit, node.MonitorStateRejoin, node.MonitorStateShutdownProgress, node.MonitorStateMaintenance} {
		w.m.ghosts.drop(web, "n2")
		w.state("n2", state)
		assert.True(t, v.IsUnsure(web), "%s", state)
	}

	// And once it is back in the cluster, until it tells the instance, or
	// something says it has none.
	w.known("n2")
	assert.True(t, v.IsUnsure(web), "the instance looked for while its node was away is one it may tell yet")
	w.full("n2", "test/svc/other")
	assert.True(t, v.IsUnsure(web), "the whole data of a node just back holds the instances it found so far")

	// The record of the delete of an object is weighed by the holder of
	// an address: the object of its id, as of the version deleted or an
	// older one, is not waited for, its node away or not. The object
	// created again under the path, or restored since, is.
	w.tombstone("n2", deleteRecord("o1", "test/svc/web", "id1", v2))
	deleted := netrule.Holder{Path: "test/svc/web", ObjectID: "id1", Version: v2}
	older := netrule.Holder{Path: "test/svc/web", ObjectID: "id1", Version: v1}
	anonymous := netrule.Holder{Path: "test/svc/web", Version: v2}
	restored := netrule.Holder{Path: "test/svc/web", ObjectID: "id1", Version: v3}
	created := netrule.Holder{Path: "test/svc/web", ObjectID: "id2", Version: v1}
	for _, state := range []node.MonitorState{node.MonitorStateIdle, node.MonitorStateRejoin} {
		w.state("n2", state)
		assert.False(t, v.view.IsUnsure(deleted), "%s: the record of its delete", state)
		assert.False(t, v.view.IsUnsure(older), "%s: an older configuration of the object deleted", state)
		assert.False(t, v.view.IsUnsure(anonymous), "%s: no id to tell it from the object deleted", state)
		assert.True(t, v.view.IsUnsure(restored), "%s: restored since", state)
		assert.True(t, v.view.IsUnsure(created), "%s: another object", state)
	}

	// A holder of no known version is not judged by the record: waited
	// for while its node is away, and no longer once the node is back
	// and tells nothing of it, which nothing else would ever say.
	assert.True(t, v.IsUnsure(web), "its node away")
	w.known("n2")
	assert.False(t, v.IsUnsure(web), "its node back, a record of its path heard")
	assert.True(t, v.view.IsUnsure(netrule.Holder{Path: "test/svc/web", ObjectID: "id2"}), "the record is of another object")

	// The record removed, the object is followed as any other. A view
	// with no memory forgets the node was away.
	w.m.onEvent(&msgbus.NodeTombstoneDeleted{Node: "n2", ID: "o1", Path: "test/svc/web"})
	assert.Empty(t, w.m.tombstones)
	assert.True(t, v.view.IsUnsure(deleted))
	w.state("n2", node.MonitorStateRejoin)
	plain := unsureView{view{heard: w.m.heard, away: w.m.isAway}}
	assert.True(t, plain.IsUnsure(web))
	assert.True(t, v.IsUnsure(web))
	w.known("n2")
	assert.False(t, plain.IsUnsure(web))
	assert.True(t, v.IsUnsure(web))
}

// What says an instance whose status went is gone, and what does not.
func TestGhosts(t *testing.T) {
	w := newWorld(t)
	db := path(t, "test/svc/db")
	gone := func() {
		instance.StatusData.Unset(db, "n2")
		w.m.onEvent(&msgbus.InstanceStatusDeleted{Path: db, Node: "n2"})
	}

	// Deleted by a node in the cluster.
	w.status("test/svc/db", "n2", "10.0.1.5")
	gone()
	assert.False(t, w.m.ghosts.has(db, "n2"))

	// Gone with the daemon of its node shutting down, and told again.
	w.status("test/svc/db", "n2", "10.0.1.5")
	w.state("n2", node.MonitorStateShutdownProgress)
	gone()
	assert.True(t, w.m.ghosts.has(db, "n2"))
	w.back("n2")
	assert.True(t, w.m.ghosts.has(db, "n2"), "a node back in the cluster has not told its instances yet")
	w.status("test/svc/db", "n2", "10.0.1.5")
	assert.False(t, w.m.ghosts.has(db, "n2"))

	// Gone with its node dropped.
	node.MonitorData.Set("n2", &node.Monitor{State: node.MonitorStateIdle})
	instance.StatusData.Unset(db, "n2")
	w.m.onEvent(&msgbus.InstanceStatusDeleted{Path: db, Node: "n2", PeerDropAt: time.Now()})
	assert.True(t, w.m.ghosts.has(db, "n2"))

	// The configuration deleted by the node in the cluster.
	w.m.onEvent(&msgbus.InstanceConfigDeleted{Path: db, Node: "n2"})
	assert.False(t, w.m.ghosts.has(db, "n2"))

	// A record of the delete of an object is no word of the instance:
	// it is of an object, which the path alone does not tell, and is
	// weighed by the address kept, see TestIsUnsure.
	w.m.ghosts.add(db, "n2")
	w.tombstone("n1", deleteRecord("o1", "test/svc/db", "id1", v1))
	assert.True(t, w.m.ghosts.has(db, "n2"))
	w.m.ghosts.drop(db, "n2")

	// An object looked for while nothing said where it runs is a ghost
	// on every node away, until its configuration says its nodes.
	w.m.ghosts.add(db, "n2")
	w.m.ghosts.add(db, "n3")
	w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: db, Node: "n2", Value: instance.Config{Scope: []string{"n2"}}})
	assert.True(t, w.m.ghosts.has(db, "n2"))
	assert.False(t, w.m.ghosts.has(db, "n3"))
	w.m.ghosts.drop(db, "n2")

	// The whole data of a node says nothing of the instances it lacks:
	// a daemon back in the cluster sends it before it found them all.
	w.m.ghosts.add(db, "n2")
	w.full("n2", "test/svc/web")
	assert.True(t, w.m.ghosts.has(db, "n2"))

	// The state judged is the one of the node when the event was
	// published, which the data of the daemon may be ahead of.
	w.m.ghosts.drop(db, "n2")
	w.m.onEvent(&msgbus.NodeMonitorUpdated{Node: "n2", Value: node.Monitor{State: node.MonitorStateMaintenance}})
	node.MonitorData.Set("n2", &node.Monitor{State: node.MonitorStateIdle})
	w.m.onEvent(&msgbus.InstanceStatusDeleted{Path: db, Node: "n2"})
	assert.True(t, w.m.ghosts.has(db, "n2"))
}

// The index: a start is indexed by the references of its rules, a stop
// drops it, and only the resources naming what changed are resolved again.
func TestTheIndexFollowsTheStartsAndTheStops(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	web := w.start("test/svc/web", "object ./svc/db\nnode n2", "")
	api := w.start("test/svc/api", "namespace prod", "object test/svc/db rid ip#1")
	plain := w.start("test/svc/plain", "node n2", "")
	require.Len(t, w.m.entries, 3)
	assert.Zero(t, web.sets+api.sets+plain.sets, "the starts set the filters the rules resolve to already")

	e := w.m.entries[w.files[path(t, "test/svc/web")][0]]
	assert.Equal(t, "app#1", e.rid)
	assert.True(t, e.names(path(t, "test/svc/db")))
	assert.False(t, e.names(path(t, "test/svc/other")))
	e = w.m.entries[w.files[path(t, "test/svc/api")][0]]
	assert.True(t, e.names(path(t, "prod/svc/anything")))
	assert.True(t, e.names(path(t, "test/svc/db")))
	assert.False(t, e.names(path(t, "test/svc/web")))

	// The stop of the resource removes its record.
	require.NoError(t, os.Remove(w.files[path(t, "test/svc/plain")][0]))
	w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: path(t, "test/svc/plain"), Node: "n1"})
	assert.Len(t, w.m.entries, 2)

	// The object is deleted from the node.
	require.NoError(t, os.Remove(w.files[path(t, "test/svc/api")][0]))
	w.m.onEvent(&msgbus.InstanceConfigDeleted{Path: path(t, "test/svc/api"), Node: "n1"})
	assert.Len(t, w.m.entries, 1)

	// The same object on another node is none of this node's.
	require.NoError(t, os.Remove(w.files[path(t, "test/svc/web")][0]))
	w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: path(t, "test/svc/web"), Node: "n2"})
	assert.Len(t, w.m.entries, 1)
}

// The filter of a running resource follows the addresses of the objects its
// rules name, set on its slice when it changes and only then.
func TestTheFilterFollowsTheObjectsNamed(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	web := w.start("test/svc/web", "object ./svc/db", "")
	other := w.start("test/svc/other", "node n2", "")
	require.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())

	// A status changing nothing of the addresses sets nothing.
	w.status("test/svc/db", "n2", "10.0.1.5")
	w.status("test/svc/unrelated", "n2", "10.0.9.9")
	w.status("prod/svc/db", "n2", "10.0.2.5")
	assert.Zero(t, web.sets)

	// A failover: the address is published by the other instance too,
	// then by it alone. Nothing changes of the address of the object.
	w.status("test/svc/db", "n1", "10.0.1.5")
	w.status("test/svc/db", "n2")
	assert.Zero(t, web.sets, "the failover of an address is no event")

	// The address changes.
	w.status("test/svc/db", "n1", "10.0.1.50")
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, "10.0.1.50/32 "+implicit, web.allowed())
	assert.Equal(t, "0.0.0.0/0 ::/0", web.denied())
	assert.True(t, w.loaded("test/svc/web").Filter.Flattened().Equal(netrule.FlatFilter(
		[]netip.Prefix{netip.MustParsePrefix("10.0.1.50/32"), netip.MustParsePrefix("10.29.0.53/32"), netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")},
		netrule.Any(),
	)), "the record is written with the filter set, for the status of the resource to follow")

	// The object stops publishing it: the status says it is gone.
	w.status("test/svc/db", "n1")
	assert.Equal(t, 2, web.sets)
	assert.Equal(t, implicit, web.allowed())

	// And is started again.
	w.status("test/svc/db", "n2", "10.0.1.5", "fd00::5")
	assert.Equal(t, 3, web.sets)
	assert.Equal(t, "10.0.1.5/32 "+implicit+" fd00::5/128", web.allowed())

	// Deleted.
	w.deleted("test/svc/db", "n1")
	w.deleted("test/svc/db", "n2")
	assert.Equal(t, implicit, web.allowed())

	assert.Zero(t, other.sets, "the resource naming no object was never set again")
}

func TestTheFilterFollowsTheNamespacesNamed(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	web := w.start("test/svc/web", "namespace .", "object ./svc/bad")
	require.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())

	w.status("test/svc/cache", "n1", "10.0.1.6")
	assert.Equal(t, "10.0.1.5/32 10.0.1.6/32 "+implicit, web.allowed())
	w.status("prod/svc/db", "n1", "10.0.2.5")
	assert.Equal(t, 1, web.sets, "an object of another namespace is none of the rule")

	// The denied object of the namespace is never let in.
	w.status("test/svc/bad", "n2", "10.0.1.7")
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, "10.0.1.5/32 10.0.1.6/32 "+implicit, web.allowed())
	w.status("test/svc/cache", "n1")
	assert.Equal(t, 2, web.sets)
	assert.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())
}

// Failing closed, the allowed side: an address goes when a status says so,
// and stays while the status of its object is not known.
func TestAnAllowedAddressIsKeptWhileItsNodeIsLost(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	web := w.start("test/svc/web", "object ./svc/db", "")

	// The daemon of n2 restarts: its data is dropped, the db runs on.
	w.lost("n2")
	assert.Zero(t, web.sets, "nothing says the address is gone")
	assert.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())
	rec := w.loaded("test/svc/web")
	assert.Equal(t, netrule.Held{netip.MustParseAddr("10.0.1.5"): {Path: "test/svc/db"}}, allowedOf(rec.Kept))

	// It is back, the db still there: between the node and its
	// instances, the address is not taken for gone.
	apply := w.back("n2", "test/svc/db=10.0.1.5")
	assert.Zero(t, web.sets)
	apply()
	assert.Zero(t, web.sets)
	assert.Empty(t, allowedOf(w.loaded("test/svc/web").Kept))

	// Lost again, and back with the db stopped.
	w.lost("n2")
	w.back("n2", "test/svc/db")()
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, implicit, web.allowed())

	// Lost with the db started again, and back without it, deleted while
	// the node was not heard of: the record of the delete says so.
	w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
	w.lost("n2")
	require.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())
	w.back("n2")()
	assert.Equal(t, "10.0.1.5/32 "+implicit, web.allowed(), "nothing says the db is gone yet")
	w.tombstone("n2", deleteRecord("o1", "test/svc/db", "id1", v1))
	assert.Equal(t, implicit, web.allowed())

	// The whole data of the node, sent as it is back in the cluster,
	// says nothing of the instances it lacks: the db created again
	// since is waited for, the record being of the one before it.
	w.runs("test/svc/db", "n2", "id2", v2, "10.0.1.5")
	w.lost("n2")
	w.back("n2")()
	require.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())
	w.full("n2", "test/svc/other")
	assert.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())
}

// The daemon of a peer restarts: it shuts down, comes back rejoining with
// no instance, then tells them as it finds them. What it runs never stopped,
// and no filter changes.
func TestNoFilterChangesWhileThePeerDaemonRestarts(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	w.status("test/svc/bad", "n2", "10.0.1.7")
	web := w.start("test/svc/web", "object ./svc/db", "")
	api := w.start("test/svc/api", "", "object ./svc/bad")

	w.state("n2", node.MonitorStateShutdownProgress)
	w.deleted("test/svc/db", "n2")
	w.deleted("test/svc/bad", "n2")
	w.state("n2", node.MonitorStateRejoin)
	assert.Zero(t, web.sets+api.sets)
	assert.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())
	assert.Equal(t, "10.0.1.7/32", api.denied())

	w.status("test/svc/db", "n2", "10.0.1.5")
	w.status("test/svc/bad", "n2", "10.0.1.7")
	w.known("n2")
	assert.Zero(t, web.sets+api.sets)
	assert.Empty(t, allowedOf(w.loaded("test/svc/web").Kept))
	assert.Empty(t, deniedOf(w.loaded("test/svc/api").Kept))

	// The daemon is in the cluster before it has told its instances.
	w.state("n2", node.MonitorStateShutdownProgress)
	w.deleted("test/svc/db", "n2")
	w.deleted("test/svc/bad", "n2")
	w.back("n2")
	assert.Zero(t, web.sets+api.sets, "back in the cluster, its instances still to be told")
	w.status("test/svc/db", "n2", "10.0.1.5")
	w.status("test/svc/bad", "n2", "10.0.1.7")
	assert.Zero(t, web.sets+api.sets)
	assert.Empty(t, allowedOf(w.loaded("test/svc/web").Kept))
}

func TestAnAllowedAddressKeptGoesToTheObjectTakingIt(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	web := w.start("test/svc/web", "object ./svc/db", "")
	w.lost("n2")
	require.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())

	// Another object is seen with the address: it is no longer the db's.
	w.status("prod/svc/squat", "n1", "10.0.1.5")
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, implicit, web.allowed())
}

// Failing closed, the denied side: an address stays denied until its object
// is deleted, or another object holds it.
func TestADeniedAddressIsKept(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/bad", "n2", "10.0.1.7")
	web := w.start("test/svc/web", "", "object ./svc/bad")
	require.Equal(t, "10.0.1.7/32", web.denied())

	// It stops publishing the address, and its node is lost: denied.
	w.status("test/svc/bad", "n2")
	assert.Zero(t, web.sets)
	assert.Equal(t, netrule.Held{netip.MustParseAddr("10.0.1.7"): {Path: "test/svc/bad"}}, deniedOf(w.loaded("test/svc/web").Kept))
	w.lost("n2")
	assert.Zero(t, web.sets)
	w.back("n2", "test/svc/bad")()
	assert.Zero(t, web.sets)

	// An object that is none of it changes: nothing to resolve again.
	e := w.m.entries[w.files[path(t, "test/svc/web")][0]]
	assert.True(t, e.keepsOf(path(t, "test/svc/bad"), nil))
	assert.False(t, e.keepsOf(path(t, "test/svc/else"), []netip.Addr{netip.MustParseAddr("10.0.9.9")}))
	assert.True(t, e.keepsOf(path(t, "test/svc/else"), []netip.Addr{netip.MustParseAddr("10.0.1.7")}))

	// It takes another address: both are denied.
	w.status("test/svc/bad", "n2", "10.0.1.8")
	assert.Equal(t, "10.0.1.7/32 10.0.1.8/32", web.denied())

	// Another object is seen holding the first.
	w.status("test/svc/good", "n1", "10.0.1.7")
	assert.Equal(t, "10.0.1.8/32", web.denied())

	// The object is deleted: nothing is denied, and the filter lifted.
	w.status("test/svc/bad", "n2")
	require.Equal(t, "10.0.1.8/32", web.denied())
	w.deleted("test/svc/bad", "n2")
	assert.Empty(t, web.deny)
	assert.Empty(t, deniedOf(w.loaded("test/svc/web").Kept))
}

// The cluster changes: the configuration of the node is read again, and
// every filter resolved again.
func TestTheFilterFollowsTheNodesAndTheNetworks(t *testing.T) {
	w := newWorld(t)
	web := w.start("test/svc/web", "node n2\nnetwork lan", "")
	require.Equal(t, "10.29.0.12/32 10.29.0.53/32 127.0.0.0/8 192.168.0.0/24 ::1/128", web.allowed())
	reads := 0
	lan := "192.168.0.0/24"
	localView = func() (netrule.View, error) {
		reads++
		return netrule.Data{
			Networks:    map[string]netrule.DataNetwork{"lan": {Ranges: []netip.Prefix{netip.MustParsePrefix(lan)}}},
			NodeAddrs:   map[string][]netip.Addr{"n1": {netip.MustParseAddr("10.29.0.11")}, "n2": {netip.MustParseAddr("10.29.0.12")}},
			Nameservers: []netip.Addr{netip.MustParseAddr("10.29.0.53")},
		}, nil
	}
	for _, event := range []any{
		&msgbus.ClusterConfigUpdated{},
		&msgbus.NodeConfigUpdated{},
		&msgbus.JoinSuccess{},
		&msgbus.ForgetPeer{},
		&msgbus.NodeNetworkStatusUpdated{},
		&msgbus.NodeNetworkStatusDeleted{},
	} {
		before := reads
		w.m.onEvent(event)
		assert.Equal(t, before+1, reads, "%T has the configuration read again", event)
	}
	assert.Zero(t, web.sets, "nothing changed of what the rules resolve to")

	lan = "192.168.8.0/24"
	w.m.onEvent(&msgbus.ClusterConfigUpdated{})
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, "10.29.0.12/32 10.29.0.53/32 127.0.0.0/8 192.168.8.0/24 ::1/128", web.allowed())

	// The events of the objects read nothing of the node.
	before := reads
	w.status("test/svc/db", "n2", "10.0.1.5")
	assert.Equal(t, before, reads)
}

// As the daemon starts, the filters of the resources running are set again,
// and what its data does not tell yet is kept as recorded.
func TestTheFiltersAreSetAgainAsTheDaemonStarts(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	w.status("test/svc/bad", "n2", "10.0.1.7")
	web := w.record("test/svc/web", "object ./svc/db\nnode n2", "object ./svc/bad")
	down := w.record("test/svc/down", "node n2", "")
	down.members = false
	want := web.allowed()

	// A daemon starting: it holds no data yet, of this node, rejoining,
	// or of its peer.
	instance.InitData()
	node.InitData()
	node.MonitorData.Set("n1", &node.Monitor{State: node.MonitorStateRejoin})
	w.m = NewManager(nil)
	w.m.log = plog.NewDefaultLogger()
	web.allow, web.deny, web.hasFilter = nil, nil, false
	w.m.onStart()
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, want, web.allowed(), "the addresses recorded are kept: nothing says they are gone")
	assert.Equal(t, "0.0.0.0/0 ::/0", web.denied())
	assert.Zero(t, down.sets, "a slice nothing runs in is set nothing")
	rec := w.loaded("test/svc/web")
	assert.Len(t, allowedOf(rec.Kept), 1)
	assert.Len(t, deniedOf(rec.Kept), 1)

	// The data fills in: the same filter, set no more.
	apply := w.back("n2", "test/svc/db=10.0.1.5", "test/svc/bad=10.0.1.7")
	w.known("n1")
	assert.Equal(t, 1, web.sets, "the peer is in the cluster, its instances not told yet")
	apply()
	assert.Equal(t, 1, web.sets)
	rec = w.loaded("test/svc/web")
	assert.Empty(t, allowedOf(rec.Kept))
	assert.Empty(t, deniedOf(rec.Kept))

	// Had the db stopped while the daemon was down, the address goes
	// when its status says so.
	w.status("test/svc/db", "n2")
	assert.Equal(t, 2, web.sets)
	assert.NotContains(t, web.allowed(), "10.0.1.5")
}

// An object of this node whose status the daemon starting has not read yet
// keeps its addresses: its configuration is on the node.
func TestADaemonStartingKeepsTheAddressesOfTheObjectsOfItsNode(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n1", "10.0.1.5")
	web := w.record("test/svc/web", "object ./svc/db", "")
	want := web.allowed()
	w.local[path(t, "test/svc/db")] = true

	instance.InitData()
	w.m = NewManager(nil)
	w.m.log = plog.NewDefaultLogger()
	w.m.onStart()
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, want, web.allowed())

	// Its status is read: stopped while the daemon was down.
	w.status("test/svc/db", "n1")
	assert.Equal(t, 2, web.sets)
	assert.Equal(t, implicit, web.allowed())
}

// A start that resolved its rules before the daemon could tell it of a
// change has its filter corrected when the daemon hears of the start.
func TestAStartIsResolvedAgain(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	web := w.record("test/svc/web", "object ./svc/db", "")
	// The address changed since, and the daemon heard of it before it
	// knew the resource.
	w.status("test/svc/db", "n2", "10.0.1.50")
	assert.Zero(t, web.sets)
	w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: path(t, "test/svc/web"), Node: "n1"})
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, "10.0.1.50/32 "+implicit, web.allowed())

	// A restart with other rules writes another record.
	web = w.start("test/svc/web", "node n2", "")
	w.status("test/svc/db", "n2", "10.0.1.51")
	assert.Zero(t, web.sets, "the rules of the last start name no object")
}

// The record a stop removed while the daemon resolved is not written back.
func TestARecordRemovedIsNotWrittenBack(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	web := w.start("test/svc/web", "object ./svc/db", "")
	file := w.files[path(t, "test/svc/web")][0]
	require.NoError(t, os.Remove(file))
	w.status("test/svc/db", "n2", "10.0.1.50")
	assert.Zero(t, web.sets)
	assert.NoFileExists(t, file)
	assert.Empty(t, w.m.entries)
}

// An object deleted is not kept for a node it never ran on: two peers
// restart one after the other, the object of the first being looked for
// while the second is away and nothing says where it runs.
func TestAnObjectDeletedAfterItsPeersRestarted(t *testing.T) {
	w := newWorld(t)
	cluster.ConfigData.Set(&cluster.Config{Nodes: []string{"n1", "n2", "n3"}})
	w.known("n3")
	set := func(addrs ...string) {
		w.status("test/svc/bad", "n2", addrs...)
		instance.ConfigData.Set(path(t, "test/svc/bad"), "n2", &instance.Config{Scope: []string{"n2"}})
		w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/bad"), Node: "n2", Value: instance.Config{Scope: []string{"n2"}}})
	}
	set("10.0.1.7")
	web := w.start("test/svc/web", "", "object ./svc/bad")
	require.Equal(t, "10.0.1.7/32", web.denied())

	w.state("n2", node.MonitorStateMaintenance)
	w.state("n3", node.MonitorStateMaintenance)
	w.deleted("test/svc/bad", "n2")
	require.True(t, w.m.ghosts.has(path(t, "test/svc/bad"), "n3"), "looked for on every node away")
	w.back("n2")
	w.back("n3")
	set("10.0.1.7")
	assert.Zero(t, web.sets)
	assert.False(t, w.m.ghosts.has(path(t, "test/svc/bad"), "n3"))

	// Deleted, its nodes in the cluster.
	w.deleted("test/svc/bad", "n2")
	assert.Empty(t, web.deny)
	assert.Empty(t, deniedOf(w.loaded("test/svc/web").Kept))
}

// The instance data of the daemon is written ahead of the events, and
// rewritten when a node sends the whole of its data: the rules are resolved
// over what the events said, so an instance missing from the data for the
// time of a few events, with no event saying so, changes no filter.
func TestTheFiltersFollowTheEventsNotTheData(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	w.status("test/svc/other", "n2", "10.0.1.6")
	web := w.start("test/svc/web", "namespace .", "")
	require.Equal(t, "10.0.1.5/32 10.0.1.6/32 "+implicit, web.allowed())

	// The data loses the db, no event says so, and an event of another
	// object of the namespace has the rules resolved again.
	instance.StatusData.Unset(path(t, "test/svc/db"), "n2")
	instance.ConfigData.Unset(path(t, "test/svc/db"), "n2")
	w.status("test/svc/other", "n2", "10.0.1.6")
	w.status("test/svc/third", "n1")
	assert.Zero(t, web.sets)
	assert.Equal(t, "10.0.1.5/32 10.0.1.6/32 "+implicit, web.allowed())
}

// TestAnAuditSelectingNetwardenAttaches pins netwarden among the audited
// subsystems: an audit naming it, or naming none, gets its logs, and one
// naming another subsystem does not, until the audit stops.
func TestAnAuditSelectingNetwardenAttaches(t *testing.T) {
	for _, tc := range []struct {
		subsystems []string
		attached   bool
	}{
		{nil, true},
		{[]string{"netwarden"}, true},
		{[]string{"net*"}, true},
		{[]string{"dns"}, false},
	} {
		w := newWorld(t)
		q := make(chan plog.LogMessage, 10)
		w.m.onEvent(&msgbus.AuditStart{Q: q, Subsystems: tc.subsystems})
		if tc.attached {
			assert.Equal(t, q, w.m.log.Q(), "%v", tc.subsystems)
			w.m.onEvent(&msgbus.AuditStop{Q: q, Subsystems: tc.subsystems})
			assert.Nil(t, w.m.log.Q(), "%v: stopped", tc.subsystems)
		} else {
			assert.Nil(t, w.m.log.Q(), "%v", tc.subsystems)
		}
	}
}

// TestAStatusSaysWhetherItsAddressesChanged pins what a status the daemon
// hears changes: its first status, and one publishing other addresses, do,
// and a status evaluated again, its addresses as they were and its other
// fields moved, does not.
func TestAStatusSaysWhetherItsAddressesChanged(t *testing.T) {
	h := make(heard)
	p := path(t, "test/svc/db")
	st := func(addrs ...string) *instance.Status {
		info := map[string]any{}
		if len(addrs) > 0 {
			info[resource.IPAddrInfoKey] = addrs[0]
			info[resource.IPAddrsInfoKey] = addrs
		}
		return &instance.Status{UpdatedAt: time.Now(), Resources: instance.ResourceStatuses{
			"ip#1": resource.Status{Status: status.Up, Info: info},
		}}
	}
	assert.True(t, h.setStatus(p, "n2", st("10.0.1.5")), "the first status")
	assert.False(t, h.setStatus(p, "n2", st("10.0.1.5")), "evaluated again, its addresses as they were")
	assert.True(t, h.setStatus(p, "n2", st("10.0.1.5", "10.0.1.6")), "an address more")
	assert.True(t, h.setStatus(p, "n2", st()), "no address")
	assert.True(t, h.setStatus(p, "n1", st()), "the first status of another node")
	h.dropStatus(p, "n2")
	assert.True(t, h.setStatus(p, "n2", st()), "a status again, after it went")
}

// The events saying the cluster may have changed, read from the
// subscription, have the node read again and every filter resolved again
// once for all of them, when no event waits: a daemon starting publishes
// them in a row. The filters resolved for another reason meanwhile are
// resolved over the node read again.
func TestTheChangesOfTheClusterReadInARowResolveTheFiltersOnce(t *testing.T) {
	w := newWorld(t)
	web := w.start("test/svc/web", "node n2\nnetwork lan", "")
	api := w.start("test/svc/api", "network lan\nobject ./svc/db", "")
	reads := 0
	lan := "192.168.0.0/24"
	localView = func() (netrule.View, error) {
		reads++
		return netrule.Data{
			Networks:    map[string]netrule.DataNetwork{"lan": {Ranges: []netip.Prefix{netip.MustParsePrefix(lan)}}},
			NodeAddrs:   map[string][]netip.Addr{"n1": {netip.MustParseAddr("10.29.0.11")}, "n2": {netip.MustParseAddr("10.29.0.12")}},
			Nameservers: []netip.Addr{netip.MustParseAddr("10.29.0.53")},
		}, nil
	}
	assert.False(t, w.m.hasDeferred())
	assert.False(t, w.m.idle(), "nothing waits")

	lan = "192.168.8.0/24"
	lines := w.logged(func() {
		for _, event := range []any{
			&msgbus.ClusterConfigUpdated{},
			&msgbus.NodeConfigUpdated{},
			&msgbus.JoinSuccess{},
			&msgbus.ForgetPeer{},
			&msgbus.NodeNetworkStatusUpdated{},
			&msgbus.NodeNetworkStatusDeleted{},
			&msgbus.NodeNetworkStatusUpdated{},
		} {
			w.m.handle(event)
			assert.True(t, w.m.hasDeferred(), "%T", event)
		}
	})
	assert.Empty(t, lines, "nothing is resolved, nor said, while events wait")
	assert.Zero(t, reads)
	assert.Zero(t, web.sets+api.sets)

	// An event of an object is followed as it is read, and resolves
	// the filter naming it over the node as it is now.
	w.m.handle(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/db"), Node: "n2", Value: instance.Config{Scope: []string{"n2"}}})
	assert.Equal(t, 1, reads)
	assert.Equal(t, 1, api.sets)
	assert.Equal(t, "10.29.0.53/32 127.0.0.0/8 192.168.8.0/24 ::1/128", api.allowed())
	st := instance.Status{Resources: instance.ResourceStatuses{
		"ip#1": resource.Status{Status: status.Up, Info: map[string]any{resource.IPAddrInfoKey: "10.0.1.5", resource.IPAddrsInfoKey: []string{"10.0.1.5"}}},
	}}
	w.m.handle(&msgbus.InstanceStatusUpdated{Path: path(t, "test/svc/db"), Node: "n2", Value: st})
	assert.Equal(t, 1, reads)
	assert.Equal(t, 2, api.sets)
	assert.Equal(t, "10.0.1.5/32 10.29.0.53/32 127.0.0.0/8 192.168.8.0/24 ::1/128", api.allowed())
	assert.Zero(t, web.sets, "no event of the object concerns it")
	assert.True(t, w.m.hasDeferred())

	// No event waits: every filter is resolved again, once, over the
	// node as it was read since the last change.
	lines = w.logged(func() { assert.True(t, w.m.idle()) })
	assert.Contains(t, lines, "debug: NodeNetworkStatusUpdated: the nodes or the networks of the cluster may have changed: read the configuration of the node again, and resolve again the 2 filter(s) of the node")
	assert.Equal(t, 1, reads, "read once for the seven events")
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, 2, api.sets, "already the filter of the node as it is")
	assert.Equal(t, "10.29.0.12/32 10.29.0.53/32 127.0.0.0/8 192.168.8.0/24 ::1/128", web.allowed())
	assert.False(t, w.m.hasDeferred())
	assert.False(t, w.m.idle())

	// The filters are resolved again before the configurations not
	// read yet are, one thing at a time.
	w.m.unread = []naming.Path{path(t, "test/svc/web")}
	w.m.handle(&msgbus.ClusterConfigUpdated{})
	assert.True(t, w.m.idle())
	assert.Len(t, w.m.unread, 1)
	assert.True(t, w.m.idle())
	assert.Empty(t, w.m.unread)
	assert.False(t, w.m.hasDeferred())
}

// The objects holding no resource have no record: theirs are not looked
// for, at the events of their instances nor as the daemon starts.
func TestTheRecordsOfTheObjectsWithNoResourceAreNotLookedFor(t *testing.T) {
	w := newWorld(t)
	globbed := make([]string, 0)
	recordFiles = func(p naming.Path) []string {
		globbed = append(globbed, p.String())
		return nil
	}
	for _, s := range []string{"test/cfg/acl", "test/sec/acl", "system/usr/admin", "test/svc/web", "test/vol/data"} {
		w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, s), Node: "n1"})
		w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: path(t, s), Node: "n1"})
	}
	assert.Equal(t, []string{"test/svc/web", "test/svc/web", "test/vol/data", "test/vol/data"}, globbed,
		"once by event, for the objects with resources")
}

// An instance of the object deleted told again while another still is, its
// node not having deleted it yet, changes nothing of what the record says:
// the address goes with the last instance telling it. And a record removed
// is forgotten.
func TestARecordOfADeleteHeardWhileItsNodeIsAwayIsRemembered(t *testing.T) {
	db := path(t, "test/svc/db")
	w := newWorld(t)
	w.runs("test/svc/db", "n1", "id1", v1, "10.0.1.5")
	w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
	g := w.guard()
	w.lost("n2")
	w.tombstone("n1", deleteRecord("o1", "test/svc/db", "id1", v1))
	require.Len(t, w.m.tombstones[db], 1)
	w.runs("test/svc/db", "n1", "id1", v1, "10.0.1.5")
	g.kept("still published by the instance of n1")
	w.deleted("test/svc/db", "n1")
	g.lifted("the record is the delete of the db, its other node heard of or not")

	// The node back without the db says nothing of it, and is away
	// again: the record is remembered, the db is not waited for.
	w.back("n2")()
	w.state("n2", node.MonitorStateMaintenance)
	g.lifted("not waited for again")
	assert.False(t, w.view().IsUnsure(netrule.Holder{Path: "test/svc/db", ObjectID: "id1", Version: v1}))

	// The record heard again, from another node holding it, is the same.
	w.tombstone("n2", deleteRecord("o1", "test/svc/db", "id1", v1))
	require.Len(t, w.m.tombstones[db], 1)

	// A record removed is forgotten.
	w.m.onEvent(&msgbus.NodeTombstoneDeleted{Node: "n1", ID: "o1", Path: "test/svc/db"})
	assert.Empty(t, w.m.tombstones)
}

// An object created under the path of one deleted is followed as any other,
// whatever the order its instances and the record of the delete of the
// first are heard in: the record names another object.
func TestAnObjectSeenAgainAfterTheRecordOfItsDeleteIsFollowedAgain(t *testing.T) {
	db := path(t, "test/svc/db")

	// The record of an object, heard while another object created under
	// its path is told, as from a node starting, is not its delete.
	w := newWorld(t)
	w.runs("test/svc/db", "n2", "id2", v2, "10.0.1.5")
	g := w.guard()
	w.tombstone("n1", deleteRecord("o1", "test/svc/db", "id1", v1))
	w.lost("n2")
	g.kept("the record is of the db before this one")
	assert.True(t, w.m.ghosts.has(db, "n2"))

	// And the configuration of another object, told while an instance of
	// the one deleted still is: the addresses are the ones of the object
	// whose instance publishes them last, which the record is not of.
	w = newWorld(t)
	w.runs("test/svc/db", "n1", "id1", v1, "10.0.1.5")
	g = w.guard()
	w.tombstone("n1", deleteRecord("o1", "test/svc/db", "id1", v1))
	w.runs("test/svc/db", "n2", "id2", v2, "10.0.1.5")
	w.deleted("test/svc/db", "n1")
	g.kept("published by the db created again")
	w.lost("n2")
	g.kept("the record is of the db before this one")
	assert.True(t, w.m.ghosts.has(db, "n2"))
}

// A daemon starting reads the records of the deletes the daemon holds, as
// it does the instances: the address recorded of an object deleted while it
// was down is not kept for a node away.
func TestADaemonStartingReadsTheRecordsOfTheDeletes(t *testing.T) {
	w := newWorld(t)
	w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
	web := w.start("test/svc/web", "object ./svc/db", "")
	require.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())

	w.restart(deleteRecord("o1", "test/svc/db", "id1", v1))
	assert.Equal(t, implicit, web.allowed())
	assert.Len(t, w.m.tombstones[path(t, "test/svc/db")], 1)
	assert.Empty(t, allowedOf(w.loaded("test/svc/web").Kept))
}
