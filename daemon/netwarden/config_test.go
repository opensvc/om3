package netwarden

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	drvnetwarden "github.com/opensvc/om3/v3/drivers/shared/netwarden"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// outdated records the instances the daemon said the status of is outdated.
type outdated []string

func (t *outdated) Pub(m pubsub.Messager, _ ...pubsub.Label) {
	if c, ok := m.(*msgbus.InstanceStateFileUpdated); ok {
		*t = append(*t, c.Path.String()+" "+c.File)
	}
}

// sliceID is the process group of the resource app#1 of an object, as the
// records of the world name it.
func sliceID(s string) string {
	return "/opensvc.slice/" + strings.ReplaceAll(s, "/", ".") + ".slice"
}

// set has the configuration of an object of this node give the rules to its
// resource app#1, as its driver reads them, with what keeps a filter from
// being set under what the resource runs, if anything.
func (w *world) set(s, allow, deny string, issue error) {
	w.t.Helper()
	p := path(w.t, s)
	dir := filepath.Join(w.dir, strings.ReplaceAll(s, "/", "."), "app#1")
	w.conf[p] = map[string]rules{"app#1": {
		net: drvnetwarden.T{Rules: listOf(allow, deny)},
		target: drvnetwarden.Target{
			Path:   p,
			PG:     &pg.Config{ID: sliceID(s)},
			VarDir: func() string { return dir },
		},
		issue: func() error { return issue },
	}}
	w.files[p] = []string{filepath.Join(dir, recordName)}
}

// configure has the configuration of an object of this node changed to the
// rules given, and the daemon hear of the change.
func (w *world) configure(s, allow, deny string) {
	w.t.Helper()
	w.set(s, allow, deny, nil)
	w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(w.t, s), Node: "n1"})
}

// said returns what the daemon logs while f runs at the info level and
// above, each line as "<level>: <message>": what it says with no audit.
func (w *world) said(f func()) []string {
	w.t.Helper()
	l := make([]string, 0)
	for _, line := range w.logged(f) {
		if !strings.HasPrefix(line, "debug: ") && !strings.HasPrefix(line, "trace: ") {
			l = append(l, line)
		}
	}
	return l
}

// logged returns what the daemon logs while f runs, each line as
// "<level>: <message>", read from the queue of an audit, without what the
// audit says of itself.
func (w *world) logged(f func()) []string {
	w.t.Helper()
	q := make(chan plog.LogMessage, 1000)
	w.m.onEvent(&msgbus.AuditStart{Q: q})
	f()
	w.m.onEvent(&msgbus.AuditStop{Q: q})
	close(q)
	l := make([]string, 0)
	for m := range q {
		if m.Message == "start auditing" || m.Message == "stop auditing" {
			continue
		}
		l = append(l, m.Level.String()+": "+m.Message)
	}
	return l
}

// A change of the rules of a resource running in a unit of its slice, the
// scope of a container or the service of an app, is set under it by the
// daemon, with no restart: tightened, loosened, and written in the record
// its status reads.
func TestAChangeOfTheRulesIsSetUnderTheRunningResource(t *testing.T) {
	w := newWorld(t)
	told := &outdated{}
	w.m.publisher = told
	web := w.start("test/svc/web", "node n2\nnetwork lan", "")
	require.Equal(t, "10.29.0.12/32 10.29.0.53/32 127.0.0.0/8 192.168.0.0/24 ::1/128", web.allowed())
	*told = nil

	// Tightened.
	lines := w.said(func() { w.configure("test/svc/web", "node n2", "") })
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, "10.29.0.12/32 "+implicit, web.allowed())
	assert.Equal(t, "0.0.0.0/0 ::/0", web.denied())
	rec := w.loaded("test/svc/web")
	assert.Equal(t, netrule.Texts{"allow node n2"}, rec.NetRules)
	assert.Equal(t, web.allowed(), strings.Join(flatAllow(rec.Filter), " "))
	assert.Equal(t, web.allowed(), strings.Join(flatAllow(rec.Public), " "))
	assert.Contains(t, lines, "info: test/svc/web app#1: rules changed: allow 10.29.0.12/32, "+implicitSteps+" on test.svc.web.slice, was allow 10.29.0.12/32, allow 192.168.0.0/24, "+implicitSteps)
	assert.Equal(t, outdated{"test/svc/web app#1/netwarden.json"}, *told, "the status of the instance reads the record")

	// The same configuration heard again, as on a change of another
	// keyword: nothing is set, nothing written.
	*told = nil
	w.configure("test/svc/web", "node n2", "")
	assert.Equal(t, 1, web.sets)
	assert.Empty(t, *told)

	// Loosened, and a deny added, which is taken out of the allowed.
	w.configure("test/svc/web", "node n2\naddress 192.168.5.0/24", "address 192.168.5.7")
	assert.Equal(t, 2, web.sets)
	assert.Equal(t, "10.29.0.12/32 10.29.0.53/32 127.0.0.0/8 192.168.5.0/30 192.168.5.4/31 192.168.5.6/32 192.168.5.8/29 192.168.5.16/28 192.168.5.32/27 192.168.5.64/26 192.168.5.128/25 ::1/128", web.allowed())
	rec = w.loaded("test/svc/web")
	assert.Equal(t, netrule.Texts{"deny address 192.168.5.7", "allow node n2", "allow address 192.168.5.0/24"}, rec.NetRules)

	// Rules written another way, resolving to the same filter: the slice
	// is set nothing, and the record holds the rules, so the status does
	// not say they wait for the daemon.
	lines = w.said(func() {
		w.configure("test/svc/web", "address 192.168.5.0/24\nnode n2\nlocalhost", "address 192.168.5.7")
	})
	assert.Equal(t, 2, web.sets)
	assert.Equal(t, netrule.Texts{"deny address 192.168.5.7", "allow address 192.168.5.0/24", "allow node n2", "allow localhost"}, w.loaded("test/svc/web").NetRules)
	require.NotEmpty(t, lines)
	assert.Contains(t, lines[len(lines)-1], "info: test/svc/web app#1: rules changed: ")
	assert.Contains(t, lines[len(lines)-1], " on test.svc.web.slice, as it was")

	// The rules follow what they name as the ones of a start do.
	w.status("test/svc/db", "n2", "10.0.1.5")
	w.configure("test/svc/web", "object ./svc/db", "")
	assert.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())
	w.status("test/svc/db", "n2", "10.0.1.50")
	assert.Equal(t, "10.0.1.50/32 "+implicit, web.allowed())
	assert.True(t, w.m.entries[w.files[path(t, "test/svc/web")][0]].names(path(t, "test/svc/db")))

	// The configuration of the object on a peer is none of this node's.
	sets := web.sets
	w.set("test/svc/web", "node n1", "", nil)
	w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/web"), Node: "n2"})
	assert.Equal(t, sets, web.sets)
}

// A running resource whose configuration loses its rules has its filter
// lifted and its record removed, as its stop would.
func TestTheRulesRemovedLiftTheFilter(t *testing.T) {
	w := newWorld(t)
	told := &outdated{}
	w.m.publisher = told
	web := w.start("test/svc/web", "node n2", "")
	require.NotEmpty(t, web.allowed())
	*told = nil

	lines := w.said(func() { w.configure("test/svc/web", "", " \n") })
	assert.Empty(t, web.allowed())
	assert.Empty(t, web.denied())
	assert.NoFileExists(t, w.files[path(t, "test/svc/web")][0])
	assert.Empty(t, w.m.entries)
	assert.Contains(t, lines, "info: test/svc/web app#1: rules removed: no filter on test.svc.web.slice, was allow 10.29.0.12/32, "+implicitSteps)
	assert.Equal(t, outdated{"test/svc/web app#1/netwarden.json"}, *told)

	// A filter that can not be lifted is kept on record, to be lifted by
	// the next change.
	api := w.start("test/svc/api", "node n2", "")
	api.setErr = errors.New("systemctl failed")
	w.configure("test/svc/api", "", "")
	assert.FileExists(t, w.files[path(t, "test/svc/api")][0])
	assert.Len(t, w.m.entries, 1)
	api.setErr = nil
	w.configure("test/svc/api", "", "")
	assert.NoFileExists(t, w.files[path(t, "test/svc/api")][0])
	assert.Empty(t, api.allowed())
}

// The rules of a keyword laid out another way, on one line or several,
// indented or not, are the same rules: the daemon sets no filter again,
// writes no record and says nothing, where it used to compare the text of
// the keyword. A rule changed among them, or moved, is still applied.
func TestALayoutOnlyChangeOfTheRulesSetsNothing(t *testing.T) {
	w := newWorld(t)
	told := &outdated{}
	w.m.publisher = told
	web := w.start("test/svc/web", "deny address 192.168.0.7\nallow node n2\nallow network lan", "")
	want := web.allowed()
	file := w.files[path(t, "test/svc/web")][0]
	before, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(before), `"rules":["deny address 192.168.0.7","allow node n2","allow network lan"]`)
	*told = nil

	for _, value := range []string{
		"deny address 192.168.0.7 allow node n2 allow network lan",
		"\n    deny address 192.168.0.7\n    allow node n2\n    allow network lan",
		"deny  address 192.168.0.7\nallow node   n2\n\n\tallow network lan\n",
	} {
		lines := w.said(func() { w.configure("test/svc/web", value, "") })
		assert.Zero(t, web.sets, "%q", value)
		assert.Empty(t, lines, "%q", value)
		assert.Empty(t, *told, "%q", value)
		assert.Equal(t, want, web.allowed(), "%q", value)
		after, err := os.ReadFile(file)
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after), "%q", value)
	}

	// The rules in another order are other rules: the record is written
	// with them, and the slice, whose two lists are the same here, is
	// set nothing.
	lines := w.said(func() {
		w.configure("test/svc/web", "deny address 192.168.0.7\nallow network lan\nallow node n2", "")
	})
	assert.Zero(t, web.sets)
	assert.Equal(t, netrule.Texts{"deny address 192.168.0.7", "allow network lan", "allow node n2"}, w.loaded("test/svc/web").NetRules)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "info: test/svc/web app#1: rules changed: ")
	assert.Contains(t, lines[0], " on test.svc.web.slice, as it was")

	// The deny moved after the allow of its network decides of nothing
	// any more: the filter is set.
	w.configure("test/svc/web", "allow network lan\ndeny address 192.168.0.7\nallow node n2", "")
	assert.Equal(t, 1, web.sets)
	assert.Contains(t, web.allowed(), "192.168.0.0/24")
	*told = nil

	// A rule taken out is a change.
	lines = w.said(func() { w.configure("test/svc/web", "\n    allow node n2", "") })
	assert.Equal(t, 2, web.sets)
	assert.NotEqual(t, want, web.allowed())
	assert.Equal(t, netrule.Texts{"allow node n2"}, w.loaded("test/svc/web").NetRules)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "info: test/svc/web app#1: rules changed: ")
	assert.Equal(t, outdated{"test/svc/web app#1/netwarden.json"}, *told)
}

// A record written by an om knowing net_allow and net_deny, as lists or as
// the text of the keywords, is followed as any other: it is read as the
// list of rules saying the same, the configuration saying those rules is
// the one of its filter, and a rule changed replaces it, the record then
// holding the one list.
func TestARecordOfTheTwoKeywordsIsFollowed(t *testing.T) {
	for name, former := range map[string]string{
		"lists": `"net_allow":["node n2","network lan"],"net_deny":["address 192.168.0.7"]`,
		"texts": `"net_allow":"\n   node n2\n   network lan","net_deny":"address 192.168.0.7"`,
	} {
		w := newWorld(t)
		// Written by a start, and not read by the daemon yet.
		web := w.record("test/svc/web", "deny address 192.168.0.7\nallow node n2\nallow network lan", "")
		want := web.allowed()
		file := w.files[path(t, "test/svc/web")][0]
		b, err := os.ReadFile(file)
		require.NoError(t, err)
		old := strings.Replace(string(b), `"rules":["deny address 192.168.0.7","allow node n2","allow network lan"]`, former, 1)
		require.NotEqual(t, string(b), old, name)
		require.NoError(t, os.WriteFile(file, []byte(old), 0o600))
		w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: path(t, "test/svc/web"), Node: "n1"})

		lines := w.said(func() {
			w.configure("test/svc/web", "\n   deny address 192.168.0.7\n   allow node n2\n   allow network lan", "")
		})
		assert.Zero(t, web.sets, name)
		assert.Empty(t, lines, name)
		assert.Equal(t, want, web.allowed(), name)

		w.configure("test/svc/web", "\n   allow node n2", "")
		assert.Equal(t, 1, web.sets, name)
		b, err = os.ReadFile(file)
		require.NoError(t, err)
		assert.Contains(t, string(b), `"rules":["allow node n2"]`, name)
		assert.NotContains(t, string(b), "net_allow", name)
		assert.NotContains(t, string(b), "net_deny", name)
	}
}

// Rules the grammar refuses, or no filter is made of, change nothing of a
// running resource: its filter is kept, and warned of, until they are
// fixed.
func TestRulesRefusedKeepTheFilter(t *testing.T) {
	w := newWorld(t)
	web := w.start("test/svc/web", "node n2", "")
	want := web.allowed()

	for _, allow := range []string{"nowhere", "node n2 port 22 proto tcp", "address from ./svc/acl key k"} {
		lines := w.said(func() { w.configure("test/svc/web", allow, "") })
		assert.Zero(t, web.sets, allow)
		assert.Equal(t, want, web.allowed(), allow)
		assert.Equal(t, netrule.Texts{"allow node n2"}, w.loaded("test/svc/web").NetRules, allow)
		require.Len(t, lines, 1, allow)
		assert.Contains(t, lines[0], "warn: test/svc/web app#1: the rules changed are refused, and the filter is kept, allow 10.29.0.12/32, "+implicitSteps+" on test.svc.web.slice: net_rules line 1: ", allow)
	}

	// Fixed.
	w.configure("test/svc/web", "node n1", "")
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, "10.29.0.11/32 "+implicit, web.allowed())

	// A filter systemd refuses leaves the rules of the one in force on
	// record, and the next change sets it.
	web.setErr = errors.New("systemctl failed")
	w.configure("test/svc/web", "node n2", "")
	assert.Equal(t, netrule.Texts{"allow node n1"}, w.loaded("test/svc/web").NetRules)
	assert.Equal(t, netrule.Texts{"allow node n1"}, w.m.entries[w.files[path(t, "test/svc/web")][0]].rec.NetRules)
	web.setErr = nil
	w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/web"), Node: "n1"})
	assert.Equal(t, netrule.Texts{"allow node n2"}, w.loaded("test/svc/web").NetRules)
	assert.Equal(t, want, web.allowed())
}

// A resource given rules while it runs with none is filtered with no
// restart when what it runs is in a unit of its slice: a container, in the
// scope of its engine, or an app started with systemd_properties.
func TestRulesGivenToAResourceRunningInAUnit(t *testing.T) {
	w := newWorld(t)
	told := &outdated{}
	w.m.publisher = told
	w.status("test/svc/db", "n2", "10.0.1.5")
	web := w.slice(sliceID("test/svc/web"))
	web.members = true

	// What would have refused the start with the rules, as an engine
	// placing its containers with cgroupfs.
	w.set("test/svc/web", "object ./svc/db", "", errors.New("docker places its containers with the cgroupfs cgroup manager"))
	lines := w.said(func() { w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/web"), Node: "n1"}) })
	assert.Zero(t, web.sets)
	assert.Empty(t, w.m.entries)
	assert.NoFileExists(t, w.files[path(t, "test/svc/web")][0])
	assert.Equal(t, []string{"warn: test/svc/web app#1: the rules are not applied to what the resource runs, unfiltered until it is restarted: docker places its containers with the cgroupfs cgroup manager"}, lines)

	// Rules refused.
	lines = w.said(func() { w.configure("test/svc/web", "nowhere", "") })
	assert.Zero(t, web.sets)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "warn: test/svc/web app#1: the rules are refused, and the resource runs unfiltered: net_rules line 1: ")

	lines = w.said(func() { w.configure("test/svc/web", "object ./svc/db", "") })
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())
	assert.Equal(t, "0.0.0.0/0 ::/0", web.denied())
	rec := w.loaded("test/svc/web")
	assert.Equal(t, netrule.Texts{"allow object ./svc/db"}, rec.NetRules)
	assert.Equal(t, sliceID("test/svc/web"), rec.PG)
	assert.Equal(t, "test", rec.Namespace)
	assert.Equal(t, netrule.Held{netip.MustParseAddr("10.0.1.5"): {Path: "test/svc/db"}}, allowedOf(rec.Held))
	assert.Contains(t, lines, "info: test/svc/web app#1: rules changed: allow 10.0.1.5/32, "+implicitSteps+" on test.svc.web.slice, was no filter")
	assert.Equal(t, outdated{"test/svc/web app#1/netwarden.json"}, *told)

	// It is followed as a resource started with its rules.
	w.status("test/svc/db", "n2", "10.0.1.50")
	assert.Equal(t, "10.0.1.50/32 "+implicit, web.allowed())
	w.configure("test/svc/web", "", "")
	assert.Empty(t, web.allowed())
	assert.Empty(t, w.m.entries)
}

// An app or a task started with no rule, and no systemd_properties, runs
// its command in no unit: systemd filters nothing of it. Rules given to it
// while it runs are warned of, and apply at its next start.
func TestRulesGivenToACommandRunningInNoUnit(t *testing.T) {
	w := newWorld(t)
	app := w.slice(sliceID("test/svc/app"))
	app.procs = true
	lines := w.said(func() { w.configure("test/svc/app", "node n2", "") })
	assert.Zero(t, app.sets)
	assert.Empty(t, w.m.entries)
	assert.NoFileExists(t, w.files[path(t, "test/svc/app")][0])
	assert.Equal(t, []string{"warn: test/svc/app app#1: the rules apply at the next start: what the resource runs is in no unit of the slice"}, lines)

	// A container its manager placed in no unit, whose driver says so,
	// as a lxc container started before om made a scope of it.
	w.set("test/svc/app", "node n2", "", errors.New("the container c1 runs out of the scope the filter is set on"))
	lines = w.said(func() { w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/app"), Node: "n1"}) })
	assert.Zero(t, app.sets)
	assert.Empty(t, w.m.entries)
	assert.Equal(t, []string{"warn: test/svc/app app#1: the rules are not applied to what the resource runs, unfiltered until it is restarted: the container c1 runs out of the scope the filter is set on"}, lines)

	// A resource not running is given nothing: its start sets its filter.
	down := w.slice(sliceID("test/svc/down"))
	lines = w.said(func() { w.configure("test/svc/down", "node n2", "") })
	assert.Zero(t, down.sets)
	assert.Empty(t, w.m.entries)
	for _, line := range lines {
		assert.NotContains(t, line, "warn: ")
	}
}

// Failing closed across a change of the rules: the addresses kept of an
// object the rules still name stay kept, and the ones of an object they no
// longer name go with their rule.
func TestTheAddressesKeptFollowTheRules(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	w.status("test/svc/bad", "n2", "10.0.1.7")
	web := w.start("test/svc/web", "object ./svc/db", "object ./svc/bad")

	// The bad object stops publishing its address, and n2 is lost: the
	// allowed address is kept, its status not known, and the denied one
	// too, its object not deleted.
	w.status("test/svc/bad", "n2")
	w.lost("n2")
	require.Equal(t, "10.0.1.5/32 "+implicit, web.allowed())
	require.Contains(t, web.denied(), "0.0.0.0/0")
	rec := w.loaded("test/svc/web")
	require.Len(t, allowedOf(rec.Kept), 1)
	require.Len(t, deniedOf(rec.Kept), 1)

	// A rule added beside them changes nothing of what is kept.
	w.configure("test/svc/web", "object ./svc/db\nnode n1", "object ./svc/bad")
	assert.Equal(t, "10.0.1.5/32 10.29.0.11/32 "+implicit, web.allowed())
	rec = w.loaded("test/svc/web")
	assert.Equal(t, netrule.Held{netip.MustParseAddr("10.0.1.5"): {Path: "test/svc/db"}}, allowedOf(rec.Kept))
	assert.Equal(t, netrule.Held{netip.MustParseAddr("10.0.1.7"): {Path: "test/svc/bad"}}, deniedOf(rec.Kept))

	// The namespace of the object named in place of it: still named.
	w.configure("test/svc/web", "namespace .", "object ./svc/bad")
	assert.Contains(t, web.allowed(), "10.0.1.5/32")
	assert.Len(t, allowedOf(w.loaded("test/svc/web").Kept), 1)

	// The rules naming them are removed: nothing of theirs is kept.
	w.configure("test/svc/web", "node n1", "address 192.168.9.9")
	assert.Equal(t, "10.29.0.11/32 "+implicit, web.allowed())
	assert.Contains(t, web.denied(), "0.0.0.0/0")
	rec = w.loaded("test/svc/web")
	assert.Empty(t, allowedOf(rec.Kept))
	assert.Empty(t, deniedOf(rec.Kept))
	assert.Empty(t, allowedOf(rec.Held))
	e := w.m.entries[w.files[path(t, "test/svc/web")][0]]
	assert.False(t, e.names(path(t, "test/svc/db")))
	assert.False(t, e.keeps())
}

// A configuration changed while the daemon was down is applied as it
// starts: the filters are set again as recorded, then the configurations
// are read, between the events, an event of a configuration reading its
// own.
func TestTheRulesChangedWhileTheDaemonWasDownAreAppliedAsItStarts(t *testing.T) {
	w := newWorld(t)
	web := w.record("test/svc/web", "node n2\nnetwork lan", "")
	gone := w.record("test/svc/gone", "node n2", "")
	same := w.record("test/svc/same", "node n2", "")
	w.set("test/svc/web", "node n2", "", nil)
	w.set("test/svc/gone", "", "", nil)
	w.set("test/svc/same", "node n2", "", nil)
	w.set("test/svc/new", "node n1", "", nil)
	fresh := w.slice(sliceID("test/svc/new"))
	fresh.members = true

	instance.InitData()
	node.InitData()
	node.MonitorData.Set("n1", &node.Monitor{State: node.MonitorStateRejoin})
	w.m = NewManager(nil)
	w.m.log = plog.NewDefaultLogger()
	w.m.onStart()
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, "10.29.0.12/32 10.29.0.53/32 127.0.0.0/8 192.168.0.0/24 ::1/128", web.allowed(), "the filter recorded, until the configuration is read")
	assert.Len(t, w.m.unread, 4)
	w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/web"), Node: "n1"})
	assert.Len(t, w.m.unread, 3)
	assert.Equal(t, 2, web.sets)
	for w.m.convergeNext() {
	}
	assert.False(t, w.m.convergeNext())

	assert.Equal(t, 2, web.sets, "read once")
	assert.Equal(t, "10.29.0.12/32 "+implicit, web.allowed())
	assert.Equal(t, netrule.Texts{"allow node n2"}, w.loaded("test/svc/web").NetRules)
	assert.Empty(t, gone.allowed())
	_, err := os.Stat(w.files[path(t, "test/svc/gone")][0])
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.Equal(t, 1, same.sets, "set again as the daemon starts, its rules unchanged")
	assert.Equal(t, 1, fresh.sets)
	assert.Equal(t, "10.29.0.11/32 "+implicit, fresh.allowed())
	assert.Len(t, w.m.entries, 3)
}

// The rules of a record changed by the daemon keep the addresses held and
// kept of the objects they still name, with the rules of the same verb
// naming them.
func TestWithRules(t *testing.T) {
	db, bad := netip.MustParseAddr("10.0.1.5"), netip.MustParseAddr("10.0.1.7")
	rec := netrule.Record{
		NetRules:  netrule.Texts{"deny object ./svc/bad", "allow object ./svc/db"},
		Namespace: "test",
		Held:      netrule.Memory{"allow object ./svc/db": {db: {Path: "test/svc/db"}}},
		Kept:      netrule.Memory{"deny object ./svc/bad": {bad: {Path: "test/svc/bad"}}},
	}
	next, err := rec.WithRules("deny namespace test\nallow object test/svc/db rid ip#1\nallow node n2")
	require.NoError(t, err)
	assert.Equal(t, netrule.Texts{"deny namespace test", "allow object test/svc/db rid ip#1", "allow node n2"}, next.NetRules)
	assert.Equal(t, netrule.Memory{"allow object test/svc/db rid ip#1": {db: {Path: "test/svc/db"}}}, next.Held)
	assert.Equal(t, netrule.Memory{"deny namespace test": {bad: {Path: "test/svc/bad"}}}, next.Kept)

	// The verbs are swapped: an address allowed of an object is none
	// denied of it.
	next, err = rec.WithRules("deny object ./svc/db\nallow object ./svc/bad")
	require.NoError(t, err)
	assert.True(t, next.Held.IsZero())
	assert.True(t, next.Kept.IsZero())

	next, err = rec.WithRules("allow nowhere")
	assert.Error(t, err)
	assert.Equal(t, rec, next)
}

// An audit shows what the daemon weighs: a line by event saying the
// resources it has resolved again, a line by resource resolved to the
// filter it has, and the records, the instances waited for and the nodes as
// they come and go. An event concerning no filter, and a status telling the
// addresses it told, as the ones evaluated again every second, are said
// nothing of. The filters set stay said at the info level.
func TestAnAuditShowsWhatIsWeighed(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")

	// A start: its record is found.
	lines := w.logged(func() { w.start("test/svc/web", "object ./svc/db", "") })
	assert.Equal(t, []string{
		"debug: test/svc/web app#1: record found, written by a start: followed, allow 10.0.1.5/32, " + implicitSteps + " on test.svc.web.slice, naming 1 object(s) and 0 namespace(s), 1 filter(s) followed",
		"debug: InstanceStatusUpdated test/svc/web@n1: resolve again test/svc/web app#1 (a resource of it)",
		"debug: test/svc/web app#1: allow 10.0.1.5/32, " + implicitSteps + " on test.svc.web.slice, unchanged",
	}, lines)

	// The object named tells the same addresses again, and another object
	// some of its own: neither is resolved again, nor said.
	lines = w.logged(func() {
		w.status("test/svc/db", "n2", "10.0.1.5")
		w.status("test/svc/db", "n2", "10.0.1.5")
		w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: path(t, "prod/svc/db"), Node: "n2"})
	})
	assert.Empty(t, lines)

	// The configuration of the object changes, its rules as they were,
	// then the one of an object with no rule.
	w.set("test/svc/web", "object ./svc/db", "", nil)
	lines = w.logged(func() {
		w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/web"), Node: "n1"})
		w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/plain"), Node: "n1"})
	})
	assert.Equal(t, []string{
		"debug: test/svc/web app#1: the rules of the configuration are the ones of its filter: unchanged",
		"debug: InstanceConfigUpdated test/svc/web@n1: resolve again test/svc/web app#1 (a resource of it)",
		"debug: test/svc/web app#1: allow 10.0.1.5/32, " + implicitSteps + " on test.svc.web.slice, unchanged",
		"debug: test/svc/plain: its configuration holds no net_rules, and none of its resources has a filter: no rule to apply",
	}, lines)

	// The peer is lost: its instance is waited for, its address kept.
	lines = w.logged(func() { w.lost("n2") })
	assert.Equal(t, []string{
		"debug: NodeMonitorDeleted n2: the node does not tell its instances any more, lost or its daemon away: resolve again the 0 filter(s) keeping addresses, of 1",
		"debug: InstanceConfigDeleted test/svc/db@n2: resolve again test/svc/web app#1 (names it)",
		"debug: test/svc/web app#1: allow 10.0.1.5/32, " + implicitSteps + " on test.svc.web.slice, unchanged",
		"debug: test/svc/db@n2: its status went while its node does not tell its instances: waited for, the addresses it held kept",
		"debug: InstanceStatusDeleted test/svc/db@n2: resolve again test/svc/web app#1 (names it)",
		"debug: test/svc/web app#1: allow 10.0.1.5/32, " + implicitSteps + " on test.svc.web.slice, unchanged: 0 allowed and 0 denied address(es) of objects held, 1 and 0 kept, was 1 allowed and 0 denied address(es) of objects held, 0 and 0 kept",
	}, lines)

	// It is back, and tells the instance stopped: the filter changes, at
	// the info level.
	w.back("n2")
	lines = w.logged(func() { w.status("test/svc/db", "n2") })
	assert.Equal(t, []string{
		"debug: test/svc/db@n2: its node tells it again: no longer waited for",
		"debug: InstanceStatusUpdated test/svc/db@n2: resolve again test/svc/web app#1 (names it)",
		"debug: test/svc/web app#1: 0 allowed and 0 denied address(es) of objects held, 0 and 0 kept, was 0 allowed and 0 denied address(es) of objects held, 1 and 0 kept",
		"info: test/svc/web app#1: " + implicitSteps + " on test.svc.web.slice, was allow 10.0.1.5/32, " + implicitSteps,
	}, lines[len(lines)-4:])

	// The networks change: every filter is resolved again.
	lines = w.logged(func() { w.m.onEvent(&msgbus.ClusterConfigUpdated{}) })
	assert.Equal(t, []string{
		"debug: ClusterConfigUpdated: the nodes or the networks of the cluster may have changed: read the configuration of the node again, and resolve again the 1 filter(s) of the node",
		"debug: test/svc/web app#1: " + implicitSteps + " on test.svc.web.slice, unchanged",
	}, lines)

	// The stop removes the record.
	require.NoError(t, os.Remove(w.files[path(t, "test/svc/web")][0]))
	lines = w.logged(func() { w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: path(t, "test/svc/web"), Node: "n1"}) })
	assert.Equal(t, []string{
		"debug: test/svc/web app#1: record gone, removed by a stop: dropped from the filters followed, 0 left",
	}, lines)
}

// A daemon starting says the records it found, and of each resource whether
// its filter is set again.
func TestAnAuditShowsTheStartOfTheDaemon(t *testing.T) {
	w := newWorld(t)
	w.record("test/svc/web", "node n2", "")
	down := w.record("test/svc/down", "node n2", "")
	down.members = false
	w.m = NewManager(nil)
	w.m.log = plog.NewDefaultLogger()
	lines := w.logged(w.m.onStart)
	assert.Contains(t, lines, "debug: starting: 2 object(s) on the node, 2 resource(s) with a filter recorded, set again if they run, 0 node(s) away: ")
	assert.Contains(t, lines, "debug: test/svc/down app#1: nothing runs in test.svc.down.slice, its record left by a stop that did not remove it: allow 10.29.0.12/32, "+implicitSteps+" is not set")
	assert.Contains(t, lines, "info: test/svc/web app#1: allow 10.29.0.12/32, "+implicitSteps+" on test.svc.web.slice, set again as the daemon starts")
}

// A key of a datastore changed is a change of the configuration of the cfg
// or the sec holding it: the rules reading it are resolved again as this
// node holds the datastore, and the filter set under the running resource.
func TestTheFilterFollowsTheKeysOfTheDatastoresItsRulesRead(t *testing.T) {
	w := newWorld(t)
	keys := map[string]map[string]string{"test/cfg/acl": {"allowed": "10.1.0.0/16\n"}}
	base, err := localView()
	require.NoError(t, err)
	localView = func() (netrule.View, error) {
		data := base.(netrule.Data)
		data.Keys = keys
		return data, nil
	}
	web := w.start("test/svc/web", "address from ./cfg/acl key allowed\naddress from ./sec/acl key peers", "")
	assert.Equal(t, "10.1.0.0/16 "+implicit, web.allowed())
	e := w.m.entries[w.files[path(t, "test/svc/web")][0]]
	assert.True(t, e.datastores[path(t, "test/cfg/acl")])
	assert.True(t, e.datastores[path(t, "test/sec/acl")])

	changed := func(s string) []string {
		return w.said(func() { w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, s), Node: "n1"}) })
	}

	// The key changes: a line added, a line that is no address.
	keys["test/cfg/acl"]["allowed"] = "10.1.0.0/16\n192.168.7.9\noops\n"
	said := changed("test/cfg/acl")
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, "10.1.0.0/16 10.29.0.53/32 127.0.0.0/8 192.168.7.9/32 ::1/128", web.allowed())
	require.Len(t, said, 1)
	assert.Contains(t, said[0], "info: test/svc/web app#1: allow 10.1.0.0/16 192.168.7.9/32, "+implicitSteps+" on test.svc.web.slice, was allow 10.1.0.0/16, ")

	// The configuration of the datastore written again, its key as it
	// was, sets nothing, and the one of a datastore no rule reads either.
	assert.Empty(t, changed("test/cfg/acl"))
	assert.Empty(t, changed("test/cfg/other"))
	assert.Equal(t, 1, web.sets)

	// The sec is created: its addresses are allowed, and the record
	// counts them.
	keys["test/sec/acl"] = map[string]string{"peers": "10.8.2.3\n"}
	changed("test/sec/acl")
	assert.Equal(t, 2, web.sets)
	assert.Contains(t, web.allowed(), "10.8.2.3/32")
	assert.Equal(t, netrule.Foreign{Secret: 1, Addrs: 1}, w.loaded("test/svc/web").Foreign)

	// The key is removed: its addresses go at once, a key being no
	// object whose status may be unknown.
	delete(keys["test/cfg/acl"], "allowed")
	changed("test/cfg/acl")
	assert.Equal(t, 3, web.sets)
	assert.Equal(t, "10.8.2.3/32 "+implicit, web.allowed())

	// The datastore is deleted.
	delete(keys, "test/sec/acl")
	w.m.onEvent(&msgbus.InstanceConfigDeleted{Path: path(t, "test/sec/acl"), Node: "n1"})
	assert.Equal(t, 4, web.sets)
	assert.Equal(t, implicit, web.allowed())
}

// A datastore of another namespace is followed as one of the namespace of
// the object: its key, and its share keyword, which is of the same
// configuration. A share that stops naming the namespace of the object
// takes the addresses of the datastore out of the running filters, the
// allowed ones and the denied ones, with a warning, and one naming it again
// gives them back, each on the event of the configuration of the datastore.
func TestTheFilterFollowsTheSharingOfTheDatastoresOfOtherNamespaces(t *testing.T) {
	w := newWorld(t)
	keys := map[string]map[string]string{
		"infra/cfg/acl":  {"allowed": "10.10.0.0/16\n"},
		"infra/cfg/bad":  {"denied": "192.168.9.9\n"},
		"prod/cfg/acl":   {"allowed": "10.9.0.0/16\n"},
		"infra/cfg/none": {"allowed": "10.12.0.0/16\n"},
	}
	shares := map[string][]string{
		"infra/cfg/acl": {"test"},
		"infra/cfg/bad": {"*"},
		"prod/cfg/acl":  {"dev"},
	}
	base, err := localView()
	require.NoError(t, err)
	localView = func() (netrule.View, error) {
		data := base.(netrule.Data)
		data.Keys, data.Shares = keys, shares
		return data, nil
	}
	web := w.start("test/svc/web",
		"address from infra/cfg/acl key allowed\naddress from prod/cfg/acl key allowed\naddress from infra/cfg/none key allowed",
		"address from infra/cfg/bad key denied")
	assert.Equal(t, "10.10.0.0/16 "+implicit, web.allowed(), "the datastores not sharing their keys with the namespace are not read")
	e := w.m.entries[w.files[path(t, "test/svc/web")][0]]
	for _, s := range []string{"infra/cfg/acl", "infra/cfg/bad", "prod/cfg/acl", "infra/cfg/none"} {
		assert.True(t, e.datastores[path(t, s)], "%s is followed, shared or not: its share keyword may change", s)
	}

	changed := func(s string) []string {
		return w.said(func() { w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, s), Node: "n1"}) })
	}

	// A key of the shared datastore changes.
	keys["infra/cfg/acl"]["allowed"] = "10.10.0.0/16\n192.168.7.9\n"
	said := changed("infra/cfg/acl")
	assert.Equal(t, 1, web.sets)
	assert.Equal(t, "10.10.0.0/16 10.29.0.53/32 127.0.0.0/8 192.168.7.9/32 ::1/128", web.allowed())
	require.NotEmpty(t, said)

	// A datastore starts sharing its keys with the namespace: its
	// addresses are allowed.
	shares["prod/cfg/acl"] = []string{"dev", "test"}
	changed("prod/cfg/acl")
	assert.Equal(t, 2, web.sets)
	assert.Contains(t, web.allowed(), "10.9.0.0/16")
	shares["infra/cfg/none"] = []string{"*"}
	changed("infra/cfg/none")
	assert.Equal(t, 3, web.sets)
	assert.Contains(t, web.allowed(), "10.12.0.0/16")

	// The share is taken back: the addresses go at once, and the
	// daemon warns of the rule it no longer resolves, once.
	shares["infra/cfg/acl"] = []string{"dev"}
	said = changed("infra/cfg/acl")
	assert.Equal(t, 4, web.sets)
	assert.Equal(t, "10.9.0.0/16 10.12.0.0/16 "+implicit, web.allowed())
	assert.Contains(t, said, "warn: test/svc/web app#1: allow address from infra/cfg/acl key allowed: the datastore infra/cfg/acl is of another namespace, and its share keyword does not share its keys with the namespace test, resolves to nothing")
	assert.Empty(t, changed("infra/cfg/acl"), "written again with nothing changed: nothing is set, and nothing warned of again")
	assert.Equal(t, 4, web.sets)

	// And so on the deny side: a datastore no longer sharing its keys
	// denies nothing, as a key removed.
	assert.Equal(t, netrule.Strings(netrule.Any()), web.deny)
	rec := w.loaded("test/svc/web")
	assert.NotContains(t, flatAllow(rec.Filter), "192.168.9.9/32")
	web2 := w.start("test/svc/open", "", "address from infra/cfg/bad key denied")
	assert.Equal(t, "192.168.9.9/32", web2.denied())
	shares["infra/cfg/bad"] = nil
	said = changed("infra/cfg/bad")
	assert.Equal(t, "", web2.denied())
	assert.Contains(t, said, "warn: test/svc/open app#1: deny address from infra/cfg/bad key denied: the datastore infra/cfg/bad is of another namespace, and its share keyword does not share its keys with the namespace test, resolves to nothing")

	// Shared again.
	shares["infra/cfg/bad"] = []string{"test"}
	changed("infra/cfg/bad")
	assert.Equal(t, "192.168.9.9/32", web2.denied())
}
