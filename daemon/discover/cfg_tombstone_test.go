package discover

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/plog"
)

func newTombstoneTestManager(t *testing.T) (*Manager, naming.Path) {
	t.Helper()
	testhelper.Setup(t)
	tombstone.InitData()
	t.Cleanup(tombstone.InitData)
	p, err := naming.ParsePath("tombtest/sec/s1")
	require.NoError(t, err)
	m := NewManager(time.Second, nil)
	m.localhost = "n1"
	m.log = plog.NewDefaultLogger()
	return m, p
}

func seedTombstone(p naming.Path, holder string, state tombstone.HolderState) {
	tombstone.Data.Set("n1", tombstone.Record{OrchestrationID: "a", Path: p.String(), Kind: tombstone.KindDeleted,
		Holders: map[string]tombstone.Holder{holder: {State: state, Mode: tombstone.HolderLive}}})
}

// A config is not fetched from a pending holder, nor fetched at all by a
// pending holder; a cleared holder is a source again.
func TestIsTombstoned(t *testing.T) {
	m, p := newTombstoneTestManager(t)
	assert.False(t, m.isTombstoned(p, "n2"))
	seedTombstone(p, "n2", tombstone.HolderPending)
	assert.True(t, m.isTombstoned(p, "n2"), "n2 holds the instance still")
	assert.False(t, m.isTombstoned(p, "n3"), "n3 is no holder")
	seedTombstone(p, "n1", tombstone.HolderPending)
	assert.True(t, m.isTombstoned(p, "n3"), "this node is a pending holder")
	seedTombstone(p, "n2", tombstone.HolderCleared)
	assert.False(t, m.isTombstoned(p, "n2"))
}

// No fetch starts from a pending holder: the node accepting the delete
// fetched the configs back from the peers whose instance had no manager
// yet, the object coming back (850 objects deleted at once on dev2).
func TestNoFetchFromAPendingHolder(t *testing.T) {
	m, p := newTombstoneTestManager(t)
	seedTombstone(p, "n2", tombstone.HolderPending)
	m.onRemoteConfigUpdated(p, "n2", instance.Config{Path: p, Scope: []string{"n1", "n2"}, UpdatedAt: time.Now()})
	assert.Empty(t, m.fetcherFrom, "no fetch from n2")
	m.fetchConfigFromRemote(fetchArgs{path: p, peer: "n2", updatedAt: time.Now()})
	assert.Empty(t, m.fetcherFrom, "the choke point refuses it too")
}

// The config of a pending holder with another scope does not remove the
// local file through the foreign config branch.
func TestAPendingHolderScopeLeavesTheLocalFile(t *testing.T) {
	m, p := newTombstoneTestManager(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\n"), 0o644))
	seedTombstone(p, "n2", tombstone.HolderPending)
	m.onRemoteConfigUpdated(p, "n2", instance.Config{Path: p, Scope: []string{"n2"}, UpdatedAt: time.Now().Add(time.Hour)})
	assert.FileExists(t, p.ConfigFile())
}

// A pending holder is no source to recover a config from.
func TestMostRecentPeerConfigSkipsAPendingHolder(t *testing.T) {
	m, p := newTombstoneTestManager(t)
	instance.ConfigData.Set(p, "n2", &instance.Config{Path: p, Scope: []string{"n1", "n2"}, UpdatedAt: time.Now()})
	t.Cleanup(func() { instance.ConfigData.Unset(p, "n2") })
	peer, _ := m.mostRecentPeerConfig(p)
	assert.Equal(t, "n2", peer)
	seedTombstone(p, "n2", tombstone.HolderPending)
	peer, cfg := m.mostRecentPeerConfig(p)
	assert.Empty(t, peer)
	assert.Nil(t, cfg)
}

// A live holder of the node is cleared once its instance is gone: not while
// its config file or its config manager is there.
func TestClearLiveHolderIfGone(t *testing.T) {
	m, p := newTombstoneTestManager(t)
	var cleared []string
	saved := clearLocalTombstoneFunc
	clearLocalTombstoneFunc = func(path string, by tombstone.ClearedBy) error {
		cleared = append(cleared, path+" "+string(by))
		return nil
	}
	t.Cleanup(func() { clearLocalTombstoneFunc = saved })
	seedTombstone(p, "n1", tombstone.HolderPending)

	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\n"), 0o600))
	m.clearLiveHolderIfGone(p)
	assert.Empty(t, cleared, "the config file is there")

	require.NoError(t, os.Remove(p.ConfigFile()))
	m.cfgMTime[p.String()] = time.Now()
	m.clearLiveHolderIfGone(p)
	assert.Empty(t, cleared, "a config manager runs")

	delete(m.cfgMTime, p.String())
	m.clearLiveHolderIfGone(p)
	assert.Equal(t, []string{p.String() + " deleted"}, cleared)
}

// A holder cleared with a config published, as one kept, is a source again:
// its config, ignored while it was pending, is fetched.
func TestClearedHolderIsASourceAgain(t *testing.T) {
	m, p := newTombstoneTestManager(t)
	// The fetch started ends at once: what is looked at is that it
	// started, from n2.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	instance.ConfigData.Set(p, "n2", &instance.Config{Path: p, Scope: []string{"n1", "n2"}, UpdatedAt: time.Now()})
	t.Cleanup(func() { instance.ConfigData.Unset(p, "n2") })
	rec := tombstone.Record{OrchestrationID: "a", Path: p.String(), Kind: tombstone.KindDeleted,
		Holders: map[string]tombstone.Holder{"n2": {State: tombstone.HolderCleared, Mode: tombstone.HolderAway, ClearedBy: tombstone.ClearedByKept}}}
	tombstone.Data.Set("n2", rec)
	m.onNodeTombstoneUpdated(&msgbus.NodeTombstoneUpdated{Node: "n2", ID: "a", Path: p.String(), Value: rec})
	assert.Equal(t, "n2", m.fetcherFrom[p.String()], "fetched from n2")
}
