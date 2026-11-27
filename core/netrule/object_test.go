package netrule

import (
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/tombstone"
)

func addrs(l ...string) []netip.Addr {
	out := make([]netip.Addr, len(l))
	for i, s := range l {
		out[i] = netip.MustParseAddr(s)
	}
	return out
}

// objectView is the cluster of testView with objects of three namespaces:
// test, the one of the object whose rules are resolved, prod and root.
func objectView() Data {
	view := testView()
	view.Objects = Objects{
		"test/svc/db": {
			"ip#1": addrs("10.0.1.5"),
			"ip#2": addrs("10.0.1.6", "fd00::6"),
			"fs#1": {},
		},
		"test/svc/web-db":  {"ip#1": addrs("10.0.1.7")},
		"test/svc/stopped": {"ip#1": {}},
		"prod/svc/db":      {"ip#1": addrs("10.0.2.5")},
		"prod/svc/api":     {"ip#1": addrs("10.0.2.6")},
		"db":               {"ip#1": addrs("10.0.3.5")},
	}
	return view
}

func resolveObjects(t *testing.T, value string) (Filter, []string) {
	t.Helper()
	rules, err := Parse(Keyword, value)
	require.NoError(t, err)
	filter, notes := Resolve(rules, objectView(), "test")
	return filter, noteTexts(notes)
}

// implicitAllowed is what the implicit rules allow over the test view.
const implicitAllowed = "10.29.0.53/32 127.0.0.0/8 ::1/128"

// allowed returns the allowed list a backend knowing no order is given.
func allowed(filter Filter) string {
	allow, _ := filter.Flat()
	return strings.Join(Strings(allow), " ")
}

// denied returns its denied list.
func denied(filter Filter) string {
	_, deny := filter.Flat()
	return strings.Join(Strings(deny), " ")
}

func TestResolveObjectRules(t *testing.T) {
	for _, c := range []struct {
		name  string
		allow string
		want  string
		notes []string
	}{
		{
			name:  "an object of the namespace of the object, every resource of it",
			allow: "allow object ./svc/db",
			want:  "10.0.1.5/32 10.0.1.6/32 " + implicitAllowed + " fd00::6/128",
		},
		{
			name:  "narrowed to a resource",
			allow: "allow object ./svc/db rid ip#2",
			want:  "10.0.1.6/32 " + implicitAllowed + " fd00::6/128",
		},
		{
			name:  "the path spelling the namespace is the same object",
			allow: "allow object test/svc/db rid ip#1",
			want:  "10.0.1.5/32 " + implicitAllowed,
		},
		{
			name:  "an object of another namespace",
			allow: "allow object prod/svc/db",
			want:  "10.0.2.5/32 " + implicitAllowed,
		},
		{
			name:  "an object of the root namespace is not one of the namespace of the object",
			allow: "allow object svc/db",
			want:  "10.0.3.5/32 " + implicitAllowed,
		},
		{
			name:  "an object that does not exist",
			allow: "allow object ./svc/foo",
			want:  implicitAllowed,
			notes: []string{"allow object ./svc/foo: no such object test/svc/foo for now, resolves to nothing until it exists"},
		},
		{
			name:  "a resource the object does not have",
			allow: "allow object ./svc/db rid ip#9",
			want:  implicitAllowed,
			notes: []string{"allow object ./svc/db rid ip#9: no such resource ip#9 in the object test/svc/db for now, resolves to nothing until it exists"},
		},
		{
			name:  "a resource publishing no address",
			allow: "allow object ./svc/db rid fs#1",
			want:  implicitAllowed,
			notes: []string{"allow object ./svc/db rid fs#1: the resource fs#1 of test/svc/db publishes no address for now, resolves to nothing until it is started"},
		},
		{
			name:  "an object not started",
			allow: "allow object ./svc/stopped",
			want:  implicitAllowed,
			notes: []string{"allow object ./svc/stopped: the object test/svc/stopped publishes no address for now, resolves to nothing until it is started"},
		},
		{
			name:  "what is no path",
			allow: "allow object ./db",
			want:  implicitAllowed,
			notes: []string{"allow object ./db: ./db is no object path, as ./svc/db or prod/svc/db, resolves to nothing"},
		},
		{
			name:  "the namespace of the object",
			allow: "allow namespace .",
			want:  "10.0.1.5/32 10.0.1.6/31 " + implicitAllowed + " fd00::6/128",
		},
		{
			name:  "another namespace",
			allow: "allow namespace prod",
			want:  "10.0.2.5/32 10.0.2.6/32 " + implicitAllowed,
		},
		{
			name:  "a namespace with no object",
			allow: "allow namespace nowhere",
			want:  implicitAllowed,
			notes: []string{"allow namespace nowhere: no object in the namespace nowhere for now, resolves to nothing until one exists"},
		},
	} {
		filter, notes := resolveObjects(t, c.allow)
		assert.Equal(t, c.want, allowed(filter), c.name)
		assert.Equal(t, "0.0.0.0/0 ::/0", denied(filter), c.name)
		if c.notes == nil {
			c.notes = []string{}
		}
		assert.Equal(t, c.notes, notes, c.name)
	}
}

// A deny rule naming an object, written before the allow rule naming its
// namespace, takes its addresses out of what that rule opens, and written
// after it decides of nothing the namespace holds.
func TestResolveObjectDenyBeforeItsNamespaceAllow(t *testing.T) {
	filter, notes := resolveObjects(t, "deny object prod/svc/api\nallow namespace prod")
	assert.Empty(t, notes)
	assert.Equal(t, "10.0.2.5/32 "+implicitAllowed, allowed(filter))

	filter, _ = resolveObjects(t, "allow namespace prod\ndeny object prod/svc/api")
	assert.Equal(t, "10.0.2.5/32 10.0.2.6/32 "+implicitAllowed, allowed(filter))

	filter, _ = resolveObjects(t, "deny object ./svc/db rid ip#1\nallow any")
	assert.Equal(t, "10.0.1.5/32", denied(filter))
	assert.Empty(t, allowed(filter))
}

func TestRuleForeign(t *testing.T) {
	rules, err := Parse(Keyword, "allow object ./svc/db\ndeny object test/svc/db\nallow object prod/svc/db\ndeny object svc/db\nallow object ./db\nallow namespace .\nallow namespace test\ndeny namespace prod\nallow node n1\nallow address 10.0.0.1")
	require.NoError(t, err)
	foreign := make([]string, 0)
	for _, rule := range rules {
		if rule.Foreign("test") {
			foreign = append(foreign, rule.Text)
		}
	}
	assert.Equal(t, []string{"allow object prod/svc/db", "deny object svc/db", "deny namespace prod"}, foreign,
		"a path spelling the namespace of the object is its ./ form, and the root namespace is another")

	objects, namespaces := References("test", rules)
	paths := make([]string, len(objects))
	for i, p := range objects {
		paths[i] = p.String()
	}
	assert.Equal(t, []string{"test/svc/db", "prod/svc/db", "db"}, paths, "each object once")
	assert.Equal(t, []string{"test", "prod"}, namespaces)

	assert.Equal(t, "object test/svc/db", rules[0].Reference("test"))
	assert.Equal(t, "namespace test", rules[5].Reference("test"))
	assert.Equal(t, "", rules[8].Reference("test"))

	db, err := naming.ParsePath("test/svc/db")
	require.NoError(t, err)
	names := make([]string, 0)
	for _, rule := range rules {
		if rule.Names(db, "test") {
			names = append(names, rule.Text)
		}
	}
	assert.Equal(t, []string{"allow object ./svc/db", "deny object test/svc/db", "allow namespace .", "allow namespace test"}, names)
}

// What is shown of a filter leaves out the addresses of the rules naming
// another namespace, and counts them.
func TestPublicHidesWhatAnotherNamespaceAdds(t *testing.T) {
	rules, err := Parse(Keyword, "deny object prod/svc/api\nallow object ./svc/db rid ip#1\nallow object prod/svc/db\nallow namespace prod\nallow node n2")
	require.NoError(t, err)
	view := objectView()
	filter, _ := Resolve(rules, view, "test")
	assert.Equal(t, "10.0.1.5/32 10.0.2.5/32 10.29.0.12/32 "+implicitAllowed, allowed(filter))

	public, foreign := Public(rules, view, "test")
	assert.Equal(t, "10.0.1.5/32 10.29.0.12/32 "+implicitAllowed, allowed(public))
	assert.Len(t, public.Steps, len(filter.Steps), "a rule whose addresses are not shown keeps its step")
	assert.Equal(t, Foreign{Rules: 3, Addrs: 2}, foreign)
	assert.Equal(t, ", and 2 addresses of 3 rules naming another namespace", foreign.Describe())
	assert.Equal(t, "", Foreign{}.Describe())
	assert.Equal(t, ", and 1 address of 1 rule naming another namespace", Foreign{Rules: 1, Addrs: 1}.Describe())

	// Only foreign allow rules: the rest is still shown denied.
	rules, err = Parse(Keyword, "allow object prod/svc/db")
	require.NoError(t, err)
	public, foreign = Public(rules, view, "test")
	assert.Equal(t, implicitAllowed, allowed(public))
	assert.Equal(t, "0.0.0.0/0 ::/0", denied(public))
	assert.Equal(t, Foreign{Rules: 1, Addrs: 1}, foreign)
}

func TestUnknownWarnsOfTheObjectsTheClusterDoesNotHave(t *testing.T) {
	rules, err := Parse(Keyword, "allow object ./svc/db\ndeny object ./svc/foo\nallow object ./svc/db rid ip#9\nallow object ./svc/stopped\nallow object prod/svc/nope\nallow namespace .\nallow namespace nowhere\nallow object ./db")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"deny object ./svc/foo: no such object test/svc/foo for now, resolves to nothing until it exists",
		"allow object ./svc/db rid ip#9: no such resource ip#9 in the object test/svc/db for now, resolves to nothing until it exists",
		"allow object ./db: ./db is no object path, as ./svc/db or prod/svc/db, resolves to nothing",
	}, noteTexts(Unknown(rules, objectView(), "test", true)), "an object not started is no warning, and nothing is said of another namespace")
	assert.Equal(t, []string{
		"allow object ./db: ./db is no object path, as ./svc/db or prod/svc/db, resolves to nothing",
	}, noteTexts(Unknown(rules, objectView(), "test", false)))
}

// The addresses are held rule by rule: an object named by two rules is held
// of each.
func TestHolding(t *testing.T) {
	rules, err := Parse(Keyword, "allow object ./svc/db rid ip#1\ndeny namespace prod\nallow node n1\ndeny object ./svc/db\nallow object ./svc/stopped")
	require.NoError(t, err)
	assert.Equal(t, Memory{
		"allow object ./svc/db rid ip#1": {netip.MustParseAddr("10.0.1.5"): {Path: "test/svc/db"}},
		"deny namespace prod": {
			netip.MustParseAddr("10.0.2.5"): {Path: "prod/svc/db"},
			netip.MustParseAddr("10.0.2.6"): {Path: "prod/svc/api"},
		},
		"deny object ./svc/db": {
			netip.MustParseAddr("10.0.1.5"): {Path: "test/svc/db"},
			netip.MustParseAddr("10.0.1.6"): {Path: "test/svc/db"},
			netip.MustParseAddr("fd00::6"):  {Path: "test/svc/db"},
		},
	}, Holding(rules, objectView(), "test"), "a rule resolving to no address holds nothing")
}

// The bookkeeping of a filter failing closed: what becomes of an address a
// rule resolved to and no longer does.
func TestHeldKeep(t *testing.T) {
	gone := netip.MustParseAddr("10.0.1.99")
	held := Held{gone: {Path: "test/svc/db"}, netip.MustParseAddr("10.0.1.5"): {Path: "test/svc/db"}}
	current := Held{netip.MustParseAddr("10.0.1.5"): {Path: "test/svc/db"}}
	kept := Held{gone: {Path: "test/svc/db"}}

	view := objectView()
	assert.Empty(t, held.Keep(current, view, false), "allowed: the status of the object says the address is gone")
	assert.Equal(t, kept, held.Keep(current, view, true), "denied: kept, the object may still answer there")

	view.Unsure = []string{"test/svc/db"}
	assert.Equal(t, kept, held.Keep(current, view, false), "allowed: kept while the status of the object is not known")
	assert.Equal(t, kept, held.Keep(current, view, true))

	// Another object publishes the address: it is that object's.
	view.Objects["prod/svc/api"] = ObjectAddrs{"ip#1": addrs("10.0.1.99")}
	assert.Empty(t, held.Keep(current, view, false))
	assert.Empty(t, held.Keep(current, view, true))

	// The object is deleted.
	view = objectView()
	delete(view.Objects, "test/svc/db")
	assert.Empty(t, held.Keep(Held{}, view, false))
	assert.Empty(t, held.Keep(Held{}, view, true), "denied: dropped with its object")

	// Deleted as far as the data goes, which does not go far: a node
	// that may hold the object is not heard of.
	view.Unsure = []string{"test/svc/db"}
	assert.Equal(t, held, held.Keep(Held{}, view, true))
	assert.Equal(t, held, held.Keep(Held{}, view, false))
}

// identView is a view telling which object each path names.
type identView struct {
	Data
	ids map[string]Holder
}

func (t identView) Identity(p naming.Path) (string, time.Time) {
	h := t.ids[p.String()]
	return h.ObjectID, h.Version
}

// addrView is a view telling which object publishes each address, and how
// many times it was asked.
type addrView struct {
	identView
	addrs map[netip.Addr]Holder
	asked *int
}

func (t addrView) AddrIdentity(p naming.Path, addr netip.Addr) (string, time.Time) {
	*t.asked++
	h := t.addrs[addr]
	return h.ObjectID, h.Version
}

// An address is held by the object publishing it, when the view tells the
// objects by address, not by the one the path is known by: an instance of
// an object deleted still told is not the object created again under its
// path. The view of a refresh asks of an address once, and an address the
// view tells no object of keeps the holder it was remembered with.
func TestTheHolderIsTheObjectPublishingTheAddress(t *testing.T) {
	v1 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	v2 := v1.Add(time.Hour)
	db := netip.MustParseAddr("10.0.1.5")
	asked := 0
	view := addrView{
		identView: identView{Data: objectView(), ids: map[string]Holder{"test/svc/db": {ObjectID: "id2", Version: v2}}},
		addrs:     map[netip.Addr]Holder{db: {ObjectID: "id1", Version: v1}},
		asked:     &asked,
	}
	const allowRule, denyRule = "allow object ./svc/db rid ip#1", "deny object ./svc/db rid ip#1"
	rules, err := Parse(Keyword, allowRule)
	require.NoError(t, err)
	want := Held{db: {Path: "test/svc/db", ObjectID: "id1", Version: v1}}
	held := Holding(rules, view, "test")
	assert.True(t, held[allowRule].Equal(want), "%v", held)

	for name, dns := range map[string][]netip.Addr{"its own nameservers": addrs("192.168.100.53"), "the nameservers of the node": nil} {
		asked = 0
		rec := Record{NetRules: Texts{denyRule, allowRule}, Namespace: "test", PG: "a.slice", DNS: dns}
		rec, _, err = rec.Refresh(view)
		require.NoError(t, err, name)
		assert.True(t, rec.Held[allowRule].Equal(want), "%s: %v", name, rec.Held)
		assert.True(t, rec.Held[denyRule].Equal(want), "%s: %v", name, rec.Held)
		assert.Equal(t, 1, asked, "%s: asked once of the address", name)

		// The configuration of the publishing instance is no longer
		// known: the holder remembered stays.
		view.addrs = nil
		rec, _, err = rec.Refresh(view)
		require.NoError(t, err, name)
		assert.True(t, rec.Held[allowRule].Equal(want), "%s: %v", name, rec.Held)
		view.addrs = map[netip.Addr]Holder{db: {ObjectID: "id1", Version: v1}}
	}
}

// The object publishing an address is remembered by its id and the version
// of its configuration, which the record of a delete is weighed against.
func TestHolderIsDeletedBy(t *testing.T) {
	v1 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	v2 := v1.Add(time.Hour)
	rec := func(path, id string, version time.Time) tombstone.Record {
		return tombstone.Record{Path: path, ObjectID: id, Version: version}
	}
	for name, tc := range map[string]struct {
		holder Holder
		rec    tombstone.Record
		want   bool
	}{
		"the delete of the holder":           {Holder{"test/svc/db", "id1", v1}, rec("test/svc/db", "id1", v1), true},
		"an older configuration of it":       {Holder{"test/svc/db", "id1", v1}, rec("test/svc/db", "id1", v2), true},
		"restored since, the same id":        {Holder{"test/svc/db", "id1", v2}, rec("test/svc/db", "id1", v1), false},
		"created again, another id":          {Holder{"test/svc/db", "id2", v2}, rec("test/svc/db", "id1", v1), false},
		"another id, whatever the versions":  {Holder{"test/svc/db", "id2", v1}, rec("test/svc/db", "id1", v2), false},
		"no id of the record, not newer":     {Holder{"test/svc/db", "id1", v1}, rec("test/svc/db", "", v1), true},
		"no id of the record, newer":         {Holder{"test/svc/db", "id1", v2}, rec("test/svc/db", "", v1), false},
		"no id of the holder, not newer":     {Holder{"test/svc/db", "", v1}, rec("test/svc/db", "id1", v1), true},
		"no id of the holder, newer":         {Holder{"test/svc/db", "", v2}, rec("test/svc/db", "id1", v1), false},
		"no version of the holder":           {Holder{Path: "test/svc/db", ObjectID: "id1"}, rec("test/svc/db", "id1", v1), false},
		"the path alone":                     {Holder{Path: "test/svc/db"}, rec("test/svc/db", "", v1), false},
		"no version of the record":           {Holder{"test/svc/db", "id1", v1}, rec("test/svc/db", "id1", time.Time{}), false},
		"another path":                       {Holder{"test/svc/db", "id1", v1}, rec("prod/svc/db", "id1", v1), false},
		"versions in another zone, the same": {Holder{"test/svc/db", "id1", v1.In(time.FixedZone("x", 3600))}, rec("test/svc/db", "id1", v1), true},
	} {
		assert.Equal(t, tc.want, tc.holder.IsDeletedBy(tc.rec), name)
	}
}

// The holders are told by a view knowing the objects, saved with the
// record, and read back, as is the path alone an om knowing no holder
// wrote.
func TestTheHoldersAreRecorded(t *testing.T) {
	v1 := time.Date(2026, 11, 1, 10, 0, 0, 123456789, time.FixedZone("x", 3600))
	db := netip.MustParseAddr("10.0.1.5")
	view := identView{Data: objectView(), ids: map[string]Holder{"test/svc/db": {ObjectID: "id1", Version: v1}}}
	const rule = "allow object ./svc/db rid ip#1"
	rules, err := Parse(Keyword, rule+"\nallow object prod/svc/db")
	require.NoError(t, err)
	held := Holding(rules, view, "test")
	assert.True(t, held.Equal(Memory{
		rule:                       {db: {Path: "test/svc/db", ObjectID: "id1", Version: v1}},
		"allow object prod/svc/db": {netip.MustParseAddr("10.0.2.5"): {Path: "prod/svc/db"}},
	}), "%v", held)

	// The view of a refresh, which answers once of an object, tells the
	// holders too, the nameservers of the resource set or not.
	rec := Record{NetRules: Texts{rule}, Namespace: "test", PG: "a.slice", DNS: addrs("192.168.100.53")}
	rec, _, err = rec.Refresh(view)
	require.NoError(t, err)
	assert.True(t, rec.Held.Equal(Memory{rule: {db: {Path: "test/svc/db", ObjectID: "id1", Version: v1}}}), "%v", rec.Held)

	// A configuration changed is another holder: the record is written.
	view.ids["test/svc/db"] = Holder{ObjectID: "id1", Version: v1.Add(time.Second)}
	next, _, err := rec.Refresh(view)
	require.NoError(t, err)
	assert.True(t, next.Filter.Equal(rec.Filter))
	assert.False(t, next.Held.Equal(rec.Held))

	// The configuration of the object no longer told, its address still
	// published, as the data of a node lost goes: the holder is the one
	// remembered, and stays so as the address is kept.
	delete(view.ids, "test/svc/db")
	next, _, err = rec.Refresh(view)
	require.NoError(t, err)
	assert.True(t, next.Held.Equal(rec.Held), "%v", next.Held)
	view.Objects["test/svc/db"]["ip#1"] = nil
	view.Unsure = []string{"test/svc/db"}
	next, _, err = next.Refresh(view)
	require.NoError(t, err)
	assert.True(t, next.Kept.Equal(rec.Held), "%v", next.Kept)
	assert.True(t, next.Held.IsZero())

	file := filepath.Join(t.TempDir(), "res", "netwarden.json")
	require.NoError(t, rec.Save(file))
	loaded, err := LoadRecord(file)
	require.NoError(t, err)
	assert.True(t, loaded.Held.Equal(rec.Held), "%v", loaded.Held)
	assert.Equal(t, "id1", loaded.Held[rule][db].ObjectID)
	assert.True(t, v1.Equal(loaded.Held[rule][db].Version), "to the nanosecond")
	b, err := os.ReadFile(file)
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &raw))
	assert.JSONEq(t, `{"allow object ./svc/db rid ip#1":{"10.0.1.5":{"path":"test/svc/db","object_id":"id1","version":"2026-11-01T10:00:00.123456789+01:00"}}}`, string(raw["held"]),
		"the addresses are recorded with their rule")

	// The holder of a path alone, as an om knowing no holder wrote it.
	old := `{"net_rules":["allow object ./svc/db"],"namespace":"test","held":{"allow object ./svc/db":{"10.0.1.5":"test/svc/db"}},"kept":{}}`
	require.NoError(t, os.WriteFile(file, []byte(old), 0o600))
	loaded, err = LoadRecord(file)
	require.NoError(t, err)
	assert.Equal(t, Memory{"allow object ./svc/db": {db: {Path: "test/svc/db"}}}, loaded.Held)
	b, err = json.Marshal(loaded.Held)
	require.NoError(t, err)
	assert.JSONEq(t, `{"allow object ./svc/db":{"10.0.1.5":{"path":"test/svc/db"}}}`, string(b), "no id and no version are not written")
}

// An address kept whose holder tells no version is given the object the
// view knows under its path, and one telling a version keeps its own.
func TestAnAddressKeptOfNoKnownVersionTakesTheObjectKnown(t *testing.T) {
	v1 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	v2 := v1.Add(time.Hour)
	gone := netip.MustParseAddr("10.0.1.99")
	view := identView{Data: objectView(), ids: map[string]Holder{"test/svc/db": {ObjectID: "id2", Version: v2}}}
	view.Unsure = []string{"test/svc/db"}

	kept := Held{gone: {Path: "test/svc/db"}}.Keep(Held{}, view, false)
	assert.True(t, kept.Equal(Held{gone: {Path: "test/svc/db", ObjectID: "id2", Version: v2}}), "%v", kept)

	known := Held{gone: {Path: "test/svc/db", ObjectID: "id1", Version: v1}}
	assert.True(t, known.Keep(Held{}, view, false).Equal(known))

	// A view telling no object leaves the holder as it is.
	plain := Held{gone: {Path: "test/svc/db"}}
	assert.Equal(t, plain, plain.Keep(Held{}, view.Data, false))
}

// plainView is a view telling nothing of what is not known: the one of a
// start.
type plainView struct{ View }

func TestRecordRefresh(t *testing.T) {
	view := objectView()
	const allowDB, allowProd, denyWeb = "allow object ./svc/db rid ip#1", "allow object prod/svc/db", "deny object ./svc/web-db"
	rec := Record{
		NetRules:  Texts{denyWeb, allowDB, allowProd},
		Namespace: "test",
		PG:        "opensvc.slice/x.slice",
	}
	// The start: a view with no memory.
	rec, notes, err := rec.Refresh(plainView{view})
	require.NoError(t, err)
	assert.Empty(t, notes)
	assert.Equal(t, "10.0.1.5/32 10.0.2.5/32 "+implicitAllowed, allowed(rec.Filter))
	assert.Equal(t, "10.0.1.5/32 "+implicitAllowed, allowed(rec.Public))
	assert.Equal(t, Foreign{Rules: 1, Addrs: 1}, rec.Foreign)
	assert.Equal(t, 2, rec.Held.Count(VerbAllow))
	assert.Equal(t, 1, rec.Held.Count(VerbDeny))
	assert.True(t, rec.Kept.IsZero())

	// The same view again: the same record.
	same, _, err := rec.Refresh(view)
	require.NoError(t, err)
	assert.True(t, same.Filter.Equal(rec.Filter))
	assert.True(t, same.Held.Equal(rec.Held))
	assert.True(t, same.Kept.Equal(rec.Kept))

	// The db moves to another address, the denied object stops: the
	// allowed address follows, the denied one is kept.
	view.Objects["test/svc/db"]["ip#1"] = addrs("10.0.1.50")
	view.Objects["test/svc/web-db"]["ip#1"] = nil
	next, _, err := rec.Refresh(view)
	require.NoError(t, err)
	assert.Equal(t, "10.0.1.50/32 10.0.2.5/32 "+implicitAllowed, allowed(next.Filter))
	assert.Equal(t, Memory{denyWeb: {netip.MustParseAddr("10.0.1.7"): {Path: "test/svc/web-db"}}}, next.Kept)
	assert.Equal(t, "deny 10.0.1.7/32", next.Filter.Steps[0].describe(), "the address kept is one of the step of its rule")

	// The denied address stays out of what an allow written after its
	// rule opens, and is still kept the time after.
	wide, err := next.WithRules(denyWeb + "\nallow address 10.0.1.0/24")
	require.NoError(t, err)
	again, _, err := wide.Refresh(view)
	require.NoError(t, err)
	assert.Equal(t, "10.0.1.0/30 10.0.1.4/31 10.0.1.6/32 10.0.1.8/29 10.0.1.16/28 10.0.1.32/27 10.0.1.64/26 10.0.1.128/25 "+implicitAllowed, allowed(again.Filter))
	assert.True(t, next.Kept.Equal(again.Kept))

	// The object of the denied address is deleted: dropped.
	delete(view.Objects, "test/svc/web-db")
	again, _, err = again.Refresh(view)
	require.NoError(t, err)
	assert.True(t, again.Kept.IsZero())
	assert.Contains(t, allowed(again.Filter), "10.0.1.0/24")

	// The status of the foreign db is lost with its node: its address
	// stays allowed, and stays hidden.
	view = objectView()
	view.Objects["prod/svc/db"]["ip#1"] = nil
	view.Unsure = []string{"prod/svc/db"}
	lost, _, err := rec.Refresh(view)
	require.NoError(t, err)
	assert.True(t, lost.Filter.Equal(rec.Filter))
	assert.Equal(t, Memory{allowProd: {netip.MustParseAddr("10.0.2.5"): {Path: "prod/svc/db"}}}, lost.Kept)
	assert.Equal(t, "10.0.1.5/32 "+implicitAllowed, allowed(lost.Public))
	assert.Equal(t, Foreign{Rules: 1, Addrs: 1}, lost.Foreign)

	// The node is back, the db is down there: the address goes.
	view.Unsure = nil
	back, _, err := lost.Refresh(view)
	require.NoError(t, err)
	assert.Equal(t, "10.0.1.5/32 "+implicitAllowed, allowed(back.Filter))
	assert.True(t, back.Kept.IsZero())

	// The nameservers of the resource are the ones recorded.
	rec.DNS = addrs("192.168.100.53")
	withDNS, _, err := rec.Refresh(objectView())
	require.NoError(t, err)
	assert.Contains(t, allowed(withDNS.Filter), "192.168.100.53/32")
	assert.NotContains(t, allowed(withDNS.Filter), "10.29.0.53/32")

	_, _, err = Record{NetRules: Texts{"allow node n1 port 22 proto tcp"}}.Refresh(view)
	assert.Error(t, err, "the cgroup backend knows no port")
	ported, _, err := Record{NetRules: Texts{"allow node n1 port 22 proto tcp"}, Backend: BackendNetdev}.Refresh(view)
	require.NoError(t, err)
	assert.Equal(t, "allow tcp/22@10.29.0.11/32 tcp/22@fd01::11/128", ported.Filter.Steps[0].describe())
}

// An object named by an allow rule and by a deny rule: its addresses are
// remembered of each rule, and what becomes of one a rule no longer
// resolves to is decided by the verb of that rule, at its place in the list.
func TestAnAddressKeptIsKeptAtThePlaceOfItsRule(t *testing.T) {
	const allowDB, denyAdmin, allowLan = "allow object ./svc/db rid ip#1", "deny object ./svc/db", "allow address 10.0.1.0/24"
	db1, db2 := netip.MustParseAddr("10.0.1.5"), netip.MustParseAddr("10.0.1.6")
	view := objectView()
	view.Objects["test/svc/db"] = ObjectAddrs{"ip#1": addrs("10.0.1.5"), "ip#2": addrs("10.0.1.6")}
	rec := Record{NetRules: Texts{allowDB, denyAdmin, allowLan}, Namespace: "test"}
	rec, _, err := rec.Refresh(view)
	require.NoError(t, err)
	assert.Equal(t, Memory{
		allowDB:   {db1: {Path: "test/svc/db"}},
		denyAdmin: {db1: {Path: "test/svc/db"}, db2: {Path: "test/svc/db"}},
	}, rec.Held)
	pass := func(rec Record, addr netip.Addr) bool {
		t.Helper()
		assert.Equal(t, firstMatch(rec.Filter, addr), flatPass(rec.Filter, addr), "%s: the two lists say what the steps do", addr)
		return firstMatch(rec.Filter, addr)
	}
	assert.True(t, pass(rec, db1), "allowed first")
	assert.False(t, pass(rec, db2), "denied before the allow of its network")
	assert.True(t, pass(rec, netip.MustParseAddr("10.0.1.9")))

	// The object stops, its status known: the allow rule gives its
	// address up, the deny rule keeps both of its own, the object still
	// existing. The address allowed first is now denied first, where a
	// kept address appended after the rules would be allowed by the
	// network.
	view.Objects["test/svc/db"] = ObjectAddrs{"ip#1": nil, "ip#2": nil}
	stopped, _, err := rec.Refresh(view)
	require.NoError(t, err)
	assert.True(t, stopped.Held.IsZero())
	assert.Equal(t, Memory{denyAdmin: {db1: {Path: "test/svc/db"}, db2: {Path: "test/svc/db"}}}, stopped.Kept)
	assert.Equal(t, 0, stopped.Kept.Count(VerbAllow))
	assert.Equal(t, 2, stopped.Kept.Count(VerbDeny))
	assert.False(t, pass(stopped, db1))
	assert.False(t, pass(stopped, db2))
	assert.True(t, pass(stopped, netip.MustParseAddr("10.0.1.9")))
	assert.Equal(t, "allow nothing, deny 10.0.1.5/32 10.0.1.6/32, allow 10.0.1.0/24, "+implicitSteps, stopped.Filter.String())

	// The same with its status unknown, its node lost: the allow rule
	// keeps its address too, at its place, before the deny.
	view.Unsure = []string{"test/svc/db"}
	lost, _, err := rec.Refresh(view)
	require.NoError(t, err)
	assert.Equal(t, Memory{
		allowDB:   {db1: {Path: "test/svc/db"}},
		denyAdmin: {db1: {Path: "test/svc/db"}, db2: {Path: "test/svc/db"}},
	}, lost.Kept)
	assert.True(t, lost.Filter.Equal(rec.Filter), "nothing moves while nothing is known")
	assert.True(t, pass(lost, db1))
	assert.False(t, pass(lost, db2))

	// The node is back and the object is deleted: both rules give up.
	view.Unsure = nil
	delete(view.Objects, "test/svc/db")
	deleted, _, err := lost.Refresh(view)
	require.NoError(t, err)
	assert.True(t, deleted.Kept.IsZero())
	assert.True(t, pass(deleted, db1), "the address is one of the network again")
	assert.True(t, pass(deleted, db2))
}

// The memory follows the rules as they change under a running resource: a
// rule moved keeps its own, and the addresses of a rule no longer written
// go to the rules of its verb still naming their object.
func TestTheMemoryOfTheRulesFollowsTheirChange(t *testing.T) {
	db, web, api := netip.MustParseAddr("10.0.1.5"), netip.MustParseAddr("10.0.1.7"), netip.MustParseAddr("10.0.2.6")
	rec := Record{
		NetRules:  Texts{"deny namespace .", "allow object prod/svc/api", "allow any"},
		Namespace: "test",
		Held: Memory{
			"deny namespace .":          {db: {Path: "test/svc/db"}},
			"allow object prod/svc/api": {api: {Path: "prod/svc/api"}},
		},
		Kept: Memory{"deny namespace .": {web: {Path: "test/svc/web-db"}}},
	}
	// Moved: the same memory.
	next, err := rec.WithRules("allow object prod/svc/api\ndeny namespace .\nallow any")
	require.NoError(t, err)
	assert.True(t, next.Held.Equal(rec.Held))
	assert.True(t, next.Kept.Equal(rec.Kept))

	// The namespace narrowed to an object of it, and the allow turned
	// into a deny: the denied addresses of the object still named stay
	// denied, the ones of the other are forgotten, and an allowed
	// address is not one denied.
	next, err = rec.WithRules("deny object ./svc/web-db\ndeny object prod/svc/api\nallow any")
	require.NoError(t, err)
	assert.True(t, next.Held.IsZero(), "%v", next.Held)
	assert.Equal(t, Memory{"deny object ./svc/web-db": {web: {Path: "test/svc/web-db"}}}, next.Kept)

	// Rules removed: nothing is remembered.
	next, err = rec.WithRules("allow any")
	require.NoError(t, err)
	assert.True(t, next.Held.IsZero())
	assert.True(t, next.Kept.IsZero())
}

func TestRecordSaveAndLoad(t *testing.T) {
	rec := Record{NetRules: Texts{"allow object ./svc/db"}, Namespace: "test", PG: "a.slice"}
	rec, _, err := rec.Refresh(objectView())
	require.NoError(t, err)
	file := filepath.Join(t.TempDir(), "res", "netwarden.json")
	require.NoError(t, rec.Save(file))
	loaded, err := LoadRecord(file)
	require.NoError(t, err)
	assert.Equal(t, rec.NetRules, loaded.NetRules)
	assert.True(t, rec.Filter.Equal(loaded.Filter))
	assert.True(t, rec.Held.Equal(loaded.Held))
	assert.Nil(t, loaded.DNS, "no nameserver of its own stays the ones of the node")
	entries, err := filepath.Glob(filepath.Join(filepath.Dir(file), "*"))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary file is left")
	entries, err = filepath.Glob(filepath.Join(filepath.Dir(file), ".*"))
	require.NoError(t, err)
	assert.Empty(t, entries)

	rec.DNS = []netip.Addr{}
	require.NoError(t, rec.Save(file))
	loaded, err = LoadRecord(file)
	require.NoError(t, err)
	assert.NotNil(t, loaded.DNS, "no nameserver at all is not the ones of the node")
}

// fickleView is a view whose objects lose their addresses once asked: the
// data of a daemon changing while the rules are resolved.
type fickleView struct {
	Data
	asked map[string]int
}

func (t fickleView) Object(p naming.Path) (ObjectAddrs, bool) {
	t.asked[p.String()]++
	if t.asked[p.String()] > 1 {
		return ObjectAddrs{}, true
	}
	return t.Data.Object(p)
}

// A record is of one state of the cluster: the filter holds the addresses
// its memory holds, whatever changes while the rules are gone over.
func TestRecordRefreshAsksEachObjectOnce(t *testing.T) {
	for _, allow := range []string{"allow object ./svc/db", "allow namespace ."} {
		view := fickleView{Data: objectView(), asked: make(map[string]int)}
		rec, _, err := Record{NetRules: Texts{"deny object ./svc/web-db", allow}, Namespace: "test"}.Refresh(view)
		require.NoError(t, err)
		for p, n := range view.asked {
			assert.Equal(t, 1, n, "%s: %s asked once", allow, p)
		}
		assert.Contains(t, rec.Held[allow], netip.MustParseAddr("10.0.1.5"), allow)
		assert.Contains(t, allowed(rec.Filter), "10.0.1.5/32", allow)
		assert.NotContains(t, allowed(rec.Filter), "10.0.1.7/32", allow)
		assert.Contains(t, rec.Held["deny object ./svc/web-db"], netip.MustParseAddr("10.0.1.7"), allow)
	}
}

// An address a rule on a port no longer resolves to is kept in the step of
// that rule, so on its port alone, also when another rule names the object
// whole.
func TestAnAddressKeptIsKeptOnThePortsOfItsRule(t *testing.T) {
	const ported, whole = "allow object ./svc/db port 5432 proto tcp", "deny object ./svc/db"
	view := objectView()
	rec := Record{NetRules: Texts{ported, whole}, Namespace: "test", Backend: BackendNetdev}
	rec, _, err := rec.Refresh(view)
	require.NoError(t, err)
	assert.Equal(t, "allow tcp/5432@10.0.1.5/32 tcp/5432@10.0.1.6/32 tcp/5432@fd00::6/128", rec.Filter.Steps[0].describe())
	view.Objects["test/svc/db"] = ObjectAddrs{"ip#1": addrs("10.0.1.5")}
	view.Unsure = []string{"test/svc/db"}
	rec, _, err = rec.Refresh(view)
	require.NoError(t, err)
	assert.Equal(t, "allow tcp/5432@10.0.1.5/32 tcp/5432@10.0.1.6/32 tcp/5432@fd00::6/128", rec.Filter.Steps[0].describe())
	assert.Equal(t, "deny 10.0.1.5/32 10.0.1.6/32 fd00::6/128", rec.Filter.Steps[1].describe())
	assert.True(t, firstMatchPort(rec.Filter, "10.0.1.6", "tcp", 5432))
	assert.False(t, firstMatchPort(rec.Filter, "10.0.1.6", "tcp", 22))
}
