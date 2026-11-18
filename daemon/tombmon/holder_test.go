package tombmon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/testhelper"
)

// newHolderState returns a state whose local node, n1, is the away holder
// of a purge of svc1, with a config file under a temporary root.
func newHolderState(t *testing.T, mode tombstone.HolderMode) (*state, naming.Path, string) {
	t.Helper()
	// Every path under a temporary root: a namespaced config is under
	// the EtcNs path, not Etc, and the test wrote in the etc of the host.
	testhelper.Setup(t)
	s, _ := newTestState(t)
	instance.InitData()
	t.Cleanup(instance.InitData)
	p := naming.Path{Namespace: "tombtest", Kind: naming.KindSvc, Name: "svc1"}
	cf := p.ConfigFile()
	require.NoError(t, os.MkdirAll(filepath.Dir(cf), 0o700))
	require.NoError(t, os.WriteFile(cf, []byte("[DEFAULT]\n"), 0o600))
	rec := tombstone.Record{OrchestrationID: id, Path: p.String(), Kind: tombstone.KindPurged,
		Holders: map[string]tombstone.Holder{"n1": pending(mode)}}
	s.upsert(rec)
	return s, p, cf
}

func touch(t *testing.T, cf string, d time.Duration) {
	t.Helper()
	at := time.Now().Add(d)
	require.NoError(t, os.Chtimes(cf, at, at))
}

// Nothing is decided before the node monitor leaves the rejoin.
func TestHolderNotBeforeRejoin(t *testing.T) {
	s, p, cf := newHolderState(t, tombstone.HolderAway)
	require.NoError(t, os.Remove(cf))
	s.evaluateLocal(p.String())
	assert.True(t, s.records[id].IsPending("n1"))
}

// An away holder whose instance is gone is cleared as purged.
func TestHolderPurgedThere(t *testing.T) {
	s, p, cf := newHolderState(t, tombstone.HolderAway)
	node.MonitorData.Set("n1", &node.Monitor{State: node.MonitorStateIdle})
	s.evaluateLocal(p.String())
	assert.True(t, s.records[id].IsPending("n1"), "the config is there")
	require.NoError(t, os.Remove(cf))
	s.evaluateLocal(p.String())
	h := s.records[id].Holders["n1"]
	assert.Equal(t, tombstone.HolderCleared, h.State)
	assert.Equal(t, tombstone.ClearedByPurged, h.ClearedBy)
}

// An away holder is kept by a change of its config while its instance is
// quiet; a change while it is purging only takes the baseline again.
func TestHolderKept(t *testing.T) {
	s, p, cf := newHolderState(t, tombstone.HolderAway)
	node.MonitorData.Set("n1", &node.Monitor{State: node.MonitorStateIdle})
	s.evaluateLocal(p.String())
	assert.True(t, s.records[id].IsPending("n1"), "the first look takes the baseline")

	instance.MonitorData.Set(p, "n1", &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectPurged, State: instance.MonitorStateIdle})
	touch(t, cf, time.Minute)
	s.evaluateLocal(p.String())
	assert.True(t, s.records[id].IsPending("n1"), "changed while purging: no keep")

	instance.MonitorData.Set(p, "n1", &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectNone, State: instance.MonitorStateIdle})
	s.evaluateLocal(p.String())
	assert.True(t, s.records[id].IsPending("n1"), "no change since the new baseline")

	touch(t, cf, 2*time.Minute)
	s.evaluateLocal(p.String())
	h := s.records[id].Holders["n1"]
	assert.Equal(t, tombstone.HolderCleared, h.State)
	assert.Equal(t, tombstone.ClearedByKept, h.ClearedBy)
}

// A live holder is left to the discovery.
func TestHolderLiveNotClearedHere(t *testing.T) {
	s, p, cf := newHolderState(t, tombstone.HolderLive)
	node.MonitorData.Set("n1", &node.Monitor{State: node.MonitorStateIdle})
	require.NoError(t, os.Remove(cf))
	s.evaluateLocal(p.String())
	assert.True(t, s.records[id].IsPending("n1"))
}

func TestReadsOnly(t *testing.T) {
	assert.True(t, readsOnly("/usr/bin/om tombtest/svc/h1 instance status -r"))
	assert.False(t, readsOnly("/usr/bin/om tombtest/svc/h1 instance unprovision"))
	assert.False(t, readsOnly("/usr/bin/om tombtest/svc/h1 instance provision"))
}

// An away holder whose instance is of another object, created again under
// the path, is cleared: it holds nothing of the one the record deleted.
func TestHolderOfAnotherObject(t *testing.T) {
	s, p, _ := newHolderState(t, tombstone.HolderAway)
	node.MonitorData.Set("n1", &node.Monitor{State: node.MonitorStateIdle})
	rec := s.records[id].DeepCopy()
	rec.ObjectID = "old"
	s.set(rec)
	instance.ConfigData.Set(p, "n1", &instance.Config{Path: p, ID: "old"})
	s.evaluateLocal(p.String())
	assert.True(t, s.records[id].IsPending("n1"), "the same object")
	instance.ConfigData.Set(p, "n1", &instance.Config{Path: p, ID: "new"})
	s.evaluateLocal(p.String())
	h := s.records[id].Holders["n1"]
	assert.Equal(t, tombstone.HolderCleared, h.State)
	assert.Equal(t, tombstone.ClearedByPurged, h.ClearedBy)
}
