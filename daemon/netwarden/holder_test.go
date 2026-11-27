package netwarden

import (
	"encoding/json"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
)

// The versions of the configurations of the tests: each is the time the
// node committing it gave it, carried with it, and none is compared with
// the clock of the test.
var (
	v1 = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	v2 = v1.Add(time.Hour)
	v3 = v2.Add(time.Hour)
)

// config has a node tell the configuration of its instance of an object,
// of the object id and the version given.
func (w *world) config(s, nodename, id string, version time.Time) {
	p := path(w.t, s)
	cfg := instance.Config{Scope: []string{"n1", "n2"}, ID: id, UpdatedAt: version}
	instance.ConfigData.Set(p, nodename, &cfg)
	w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: p, Node: nodename, Value: cfg})
}

// runs has a node tell its instance of an object, of the object id and the
// version given, publishing the addresses given.
func (w *world) runs(s, nodename, id string, version time.Time, addrs ...string) {
	w.config(s, nodename, id, version)
	w.status(s, nodename, addrs...)
}

// deleteRecord returns the record of the delete of the object of the id
// given, as of the version given.
func deleteRecord(orchestration, s, id string, version time.Time) tombstone.Record {
	return tombstone.Record{OrchestrationID: orchestration, Path: s, Kind: tombstone.KindDeleted, ObjectID: id, Version: version}
}

// tombstone has the daemon hear a record of a delete from a node.
func (w *world) tombstone(nodename string, rec tombstone.Record) {
	tombstone.Data.Set(nodename, rec, false)
	w.m.onEvent(&msgbus.NodeTombstoneUpdated{Node: nodename, ID: rec.OrchestrationID, Path: rec.Path, Value: rec})
}

// restart has the daemon of this node start again, knowing nothing of its
// peer, and holding the records of deletes given.
func (w *world) restart(records ...tombstone.Record) {
	instance.InitData()
	node.InitData()
	tombstone.InitData()
	node.MonitorData.Set("n1", &node.Monitor{State: node.MonitorStateIdle})
	for _, rec := range records {
		tombstone.Data.Set("n1", rec, false)
	}
	w.m = NewManager(nil)
	w.m.log = plog.NewDefaultLogger()
	w.m.onStart()
}

// legacy rewrites the record of a resource as an om knowing no holder wrote
// it: each address held or kept with the path of its object alone.
func (w *world) legacy(s string) {
	w.t.Helper()
	file := w.files[path(w.t, s)][0]
	b, err := os.ReadFile(file)
	require.NoError(w.t, err)
	var data map[string]any
	require.NoError(w.t, json.Unmarshal(b, &data))
	for _, memory := range []string{"held", "kept"} {
		sides, _ := data[memory].(map[string]any)
		for _, side := range sides {
			addrs, _ := side.(map[string]any)
			for addr, holder := range addrs {
				if m, ok := holder.(map[string]any); ok {
					addrs[addr] = m["path"]
				}
			}
		}
	}
	b, err = json.Marshal(data)
	require.NoError(w.t, err)
	require.NoError(w.t, os.WriteFile(file, b, 0o600))
}

// guarded is the two resources of this node naming the db, one allowing it
// and one denying it, and what their filters hold of its address.
type guarded struct {
	w           *world
	allow, deny *fakeSlice
}

const dbAddr = "10.0.1.5/32"

// guard starts the two resources.
func (w *world) guard() guarded {
	w.t.Helper()
	g := guarded{w: w}
	g.allow = w.start("test/svc/web", "object ./svc/db", "")
	g.deny = w.start("test/svc/api", "", "object ./svc/db")
	g.kept("as started")
	return g
}

// kept says the address of the db is allowed by the one and denied by the
// other.
func (g guarded) kept(msg string) {
	g.w.t.Helper()
	assert.Equal(g.w.t, dbAddr+" "+implicit, g.allow.allowed(), "allowed: %s", msg)
	assert.Equal(g.w.t, dbAddr, g.deny.denied(), "denied: %s", msg)
}

// lifted says the address of the db is in neither filter, nor remembered.
func (g guarded) lifted(msg string) {
	g.w.t.Helper()
	assert.Equal(g.w.t, implicit, g.allow.allowed(), "allowed: %s", msg)
	assert.Empty(g.w.t, g.deny.deny, "denied: %s", msg)
	assert.Empty(g.w.t, allowedOf(g.w.loaded("test/svc/web").Kept), "allowed: %s", msg)
	assert.Empty(g.w.t, deniedOf(g.w.loaded("test/svc/api").Kept), "denied: %s", msg)
}

// A record of a delete heard while an instance of the path is still told,
// its node in maintenance, is not the delete of the object restored since
// with a newer configuration, of the same id or of one the record or the
// configuration does not tell: its addresses are kept, allowed and denied,
// when its node is lost.
func TestARecordIsNotTheDeleteOfAConfigurationRestoredSince(t *testing.T) {
	for name, ids := range map[string][2]string{
		"same id":                    {"id1", "id1"},
		"id unknown of the record":   {"", "id1"},
		"id unknown of the instance": {"id1", ""},
		"id unknown of both":         {"", ""},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			w.runs("test/svc/db", "n2", ids[1], v1, "10.0.1.5")
			g := w.guard()
			w.state("n2", node.MonitorStateMaintenance)
			w.tombstone("n1", deleteRecord("o1", "test/svc/db", ids[0], v1))
			g.kept("the instance is still told")

			// Restored, its configuration newer than the one deleted.
			w.config("test/svc/db", "n2", ids[1], v2)
			g.kept("restored")
			w.lost("n2")
			g.kept("the record is of the configuration before the one restored")
			assert.True(t, w.m.ghosts.has(path(t, "test/svc/db"), "n2"))
			assert.Len(t, allowedOf(w.loaded("test/svc/web").Kept), 1)
			assert.Len(t, deniedOf(w.loaded("test/svc/api").Kept), 1)

			// Back with the db: nothing moved.
			w.back("n2")
			w.runs("test/svc/db", "n2", ids[1], v2, "10.0.1.5")
			g.kept("told again")
			assert.Zero(t, g.allow.sets+g.deny.sets)
		})
	}
}

// An object is deleted, and created again or restored under its path: a
// node tells the configuration of the new one, publishing nothing yet,
// while another still tells its instance of the object deleted, publishing
// its address. The address is the one of the object deleted, not of the
// newest configuration heard of the path: the record of the delete lifts it
// from the allowed ones when its node is lost before it says the instance
// is gone, where it was kept until the node told the path again. A denied
// address stays, the path naming an object.
func TestAStatusOfTheObjectDeletedIsNotTheOneOfTheConfigurationHeardSince(t *testing.T) {
	for name, id := range map[string]string{
		"created again, another id": "id2",
		"restored, the same id":     "id1",
	} {
		t.Run(name, func(t *testing.T) {
			addr := netip.MustParseAddr("10.0.1.5")
			deletedOne := netrule.Holder{Path: "test/svc/db", ObjectID: "id1", Version: v1}
			w := newWorld(t)
			w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
			g := w.guard()

			// The new object, on this node, with no address yet.
			w.runs("test/svc/db", "n1", id, v2)
			g.kept("still published by the instance of the object deleted")
			assert.Equal(t, deletedOne, allowedOf(w.loaded("test/svc/web").Held)[addr], "allowed: the object publishing the address")
			assert.Equal(t, deletedOne, deniedOf(w.loaded("test/svc/api").Held)[addr], "denied: the object publishing the address")

			w.tombstone("n1", deleteRecord("o1", "test/svc/db", "id1", v1))
			g.kept("the instance is still told")

			// Lost before it tells its instance deleted: its
			// configuration goes first, then its status.
			w.lost("n2")
			assert.Equal(t, implicit, g.allow.allowed(), "allowed: the record is the delete of the object that published the address")
			assert.Empty(t, allowedOf(w.loaded("test/svc/web").Kept))
			assert.Equal(t, dbAddr, g.deny.denied(), "denied: the path names an object")
			assert.Equal(t, deletedOne, deniedOf(w.loaded("test/svc/api").Kept)[addr])

			// Back, with the new object: nothing to lift then.
			w.back("n2")
			w.runs("test/svc/db", "n2", id, v2)
			assert.Equal(t, implicit, g.allow.allowed())
			assert.Equal(t, dbAddr, g.deny.denied())
		})
	}
}

// A resource starts while a node back from a delete still tells its
// instance of the object deleted, publishing its address, beside the object
// created again under the path, publishing another: the start records each
// address with the object publishing it, as the daemon told it, not both
// with the newest configuration of the path. The node lost before the daemon
// finds the record of the start, the record of the delete lifts the address
// of the object deleted, where it was kept allowed for as long as the node
// was away.
func TestAStartRecordsTheObjectPublishingEachAddress(t *testing.T) {
	older, newer := netip.MustParseAddr("10.0.1.5"), netip.MustParseAddr("10.0.1.6")
	w := newWorld(t)
	w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
	w.runs("test/svc/db", "n1", "id2", v2, "10.0.1.6")
	w.tombstone("n1", deleteRecord("o1", "test/svc/db", "id1", v1))

	// The record of the start, which the daemon has not found yet.
	fs := w.record("test/svc/web", "object ./svc/db", "")
	held := allowedOf(w.loaded("test/svc/web").Held)
	assert.Equal(t, netrule.Holder{Path: "test/svc/db", ObjectID: "id1", Version: v1}, held[older], "the address of the object deleted")
	assert.Equal(t, netrule.Holder{Path: "test/svc/db", ObjectID: "id2", Version: v2}, held[newer], "the address of the object created again")

	// Lost before the daemon resolves the rules of the start again.
	w.lost("n2")
	w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: path(t, "test/svc/web"), Node: "n1"})
	assert.Equal(t, "10.0.1.6/32 "+implicit, fs.allowed(), "the record is the delete of the object that published the address")
	assert.Empty(t, allowedOf(w.loaded("test/svc/web").Kept))
}

// An address the instances of two nodes publish is the one of the newest
// of their configurations: a node holding the older one is about to fetch
// the other, and the record of the delete of the older is not the one of
// the object restored.
func TestAnAddressPublishedByTwoInstancesIsOfTheNewestOfTheirConfigurations(t *testing.T) {
	addr := netip.MustParseAddr("10.0.1.5")
	w := newWorld(t)
	w.runs("test/svc/db", "n1", "id1", v1, "10.0.1.5")
	w.runs("test/svc/db", "n2", "id1", v2, "10.0.1.5")
	w.start("test/svc/web", "object ./svc/db", "")
	assert.Equal(t, netrule.Holder{Path: "test/svc/db", ObjectID: "id1", Version: v2}, allowedOf(w.loaded("test/svc/web").Held)[addr])
}

// A daemon starting holds the record of the delete of an object created
// again since, or restored, on a node it does not hear of yet: the
// addresses recorded of the new object are not the ones of the object
// deleted, which the record says without the node being heard.
func TestADaemonStartingKeepsTheAddressesOfAnObjectCreatedAgain(t *testing.T) {
	for name, tc := range map[string]struct {
		id      string
		version time.Time
	}{
		"created again, another id":  {"id2", v2},
		"restored, the same id":      {"id1", v2},
		"created again, no id known": {"", v2},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			w.runs("test/svc/db", "n2", tc.id, tc.version, "10.0.1.5")
			g := w.guard()

			w.restart(deleteRecord("o1", "test/svc/db", "id1", v1))
			g.kept("the record is of the object before this one")

			// The peer is heard, its record first: its daemon reads
			// the records before it discovers its instances.
			w.back("n2")
			w.tombstone("n2", deleteRecord("o1", "test/svc/db", "id1", v1))
			g.kept("the peer has not told the db yet")
			w.runs("test/svc/db", "n2", tc.id, tc.version, "10.0.1.5")
			g.kept("told")
			assert.Empty(t, allowedOf(w.loaded("test/svc/web").Kept))
			assert.Empty(t, deniedOf(w.loaded("test/svc/api").Kept))
		})
	}
}

// The two daemons restarted: the peer tells the record it holds before its
// instances. The addresses this node remembers of the object created again
// are not the ones of the record.
func TestARecordToldBeforeTheInstancesOfItsNode(t *testing.T) {
	w := newWorld(t)
	w.runs("test/svc/db", "n2", "id2", v2, "10.0.1.5")
	g := w.guard()

	w.restart()
	g.kept("the peer is not heard of")
	w.back("n2")
	g.kept("the peer has not told the db yet")
	w.tombstone("n2", deleteRecord("o1", "test/svc/db", "id1", v1))
	g.kept("the record is of the object before this one")
	assert.True(t, w.m.ghosts.has(path(t, "test/svc/db"), "n2"))
	w.runs("test/svc/db", "n2", "id2", v2, "10.0.1.5")
	g.kept("told")
	assert.Empty(t, allowedOf(w.loaded("test/svc/web").Kept))
	assert.Empty(t, deniedOf(w.loaded("test/svc/api").Kept))
}

// The delete of an object lifts its addresses of both sides at once: its
// record names the object and the version the filters remember.
func TestTheDeleteOfAnObjectLiftsItsAddresses(t *testing.T) {
	rec := deleteRecord("o1", "test/svc/db", "id1", v1)

	t.Run("its node away", func(t *testing.T) {
		w := newWorld(t)
		w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
		g := w.guard()
		w.lost("n2")
		g.kept("nothing says the db is gone")
		w.tombstone("n1", rec)
		g.lifted("the record is the delete of the db")
		w.back("n2")()
		g.lifted("the node back says nothing of the db")
	})

	t.Run("its node back, then the record", func(t *testing.T) {
		w := newWorld(t)
		w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
		g := w.guard()
		w.lost("n2")
		w.back("n2")()
		g.kept("nothing says the db is gone yet")
		w.tombstone("n2", rec)
		g.lifted("the record is the delete of the db")
	})

	t.Run("an older configuration of the object", func(t *testing.T) {
		w := newWorld(t)
		w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
		g := w.guard()
		w.lost("n2")
		w.tombstone("n1", deleteRecord("o1", "test/svc/db", "id1", v2))
		g.lifted("the configuration deleted is newer than the one remembered")
	})

	t.Run("its node in the cluster", func(t *testing.T) {
		w := newWorld(t)
		w.runs("test/svc/db", "n1", "id1", v1, "10.0.1.5")
		w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
		g := w.guard()
		w.tombstone("n1", rec)
		w.deleted("test/svc/db", "n1")
		g.kept("still published by the instance of n2")
		w.deleted("test/svc/db", "n2")
		g.lifted("deleted by its nodes")
	})

	t.Run("the record, then its node lost", func(t *testing.T) {
		w := newWorld(t)
		w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
		g := w.guard()
		w.state("n2", node.MonitorStateMaintenance)
		w.tombstone("n1", rec)
		g.kept("the status of the instance is still told")
		w.lost("n2")
		g.lifted("the record is the delete of the db")
	})

	t.Run("as the daemon starts", func(t *testing.T) {
		w := newWorld(t)
		w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
		g := w.guard()
		w.restart(rec)
		g.lifted("the record is the delete of the db recorded")
	})
}

// An object created again under the path of one deleted, with another id,
// keeps its addresses while its node is not heard of: the record of the
// delete of the first is none of it, heard again or not, and its own delete
// is another record.
func TestAnObjectCreatedAgainKeepsItsAddresses(t *testing.T) {
	w := newWorld(t)
	w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
	g := w.guard()
	w.lost("n2")
	w.tombstone("n1", deleteRecord("o1", "test/svc/db", "id1", v1))
	g.lifted("deleted")

	w.back("n2")
	w.runs("test/svc/db", "n2", "id2", v2, "10.0.1.5")
	g.kept("created again")
	w.lost("n2")
	g.kept("nothing says the db created again is gone")
	w.tombstone("n2", deleteRecord("o1", "test/svc/db", "id1", v1))
	g.kept("the record heard again is of the db before")
	w.tombstone("n1", deleteRecord("o2", "test/svc/db", "id2", v2))
	g.lifted("deleted again")
}

// A record written by an om knowing no holder names the path of the object
// alone: a record of a delete does not lift its addresses by itself, while
// the node of the object is not heard of. The node back, and telling
// nothing of the object, it does: the addresses are not kept for good.
func TestARecordOfAFilterNamingNoHolder(t *testing.T) {
	rec := deleteRecord("o1", "test/svc/db", "id1", v1)

	t.Run("deleted", func(t *testing.T) {
		w := newWorld(t)
		w.runs("test/svc/db", "n2", "id1", v1, "10.0.1.5")
		g := w.guard()
		w.legacy("test/svc/web")
		w.legacy("test/svc/api")
		w.restart(rec)
		g.kept("nothing says which object the addresses were of")
		w.back("n2")()
		g.lifted("the node is back, and tells nothing of the db")
	})

	t.Run("created again", func(t *testing.T) {
		w := newWorld(t)
		w.runs("test/svc/db", "n2", "id2", v2, "10.0.1.5")
		g := w.guard()
		w.legacy("test/svc/web")
		w.legacy("test/svc/api")
		w.restart(rec)
		g.kept("nothing says which object the addresses were of")
		w.back("n2")
		w.runs("test/svc/db", "n2", "id2", v2, "10.0.1.5")
		g.kept("told")
		w.lost("n2")
		g.kept("the addresses are known of the db created again")
		assert.Len(t, allowedOf(w.loaded("test/svc/web").Kept), 1)
		assert.Len(t, deniedOf(w.loaded("test/svc/api").Kept), 1)
	})
}
