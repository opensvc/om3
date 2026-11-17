package discover

import (
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/daemon/tombmon"
	"github.com/opensvc/om3/v3/util/file"
)

// clearLocalTombstoneFunc clears the holder of the node in the tombstones of
// a path: a variable for the tests to stub.
var clearLocalTombstoneFunc = tombmon.ClearLocal

// isTombstoned says a config of p is not to be fetched from peer: peer is a
// pending holder of a tombstone of p, its instance not deleted yet, so the
// object would come back from it; or this node is, its instance to be
// deleted, so it fetches nothing of the path, which keeps the change of its
// config file a local signal.
func (t *Manager) isTombstoned(p naming.Path, peer string) bool {
	for _, rec := range tombstone.Data.MergedOf(p.String()) {
		if rec.IsPending(t.localhost) || (peer != "" && rec.IsPending(peer)) {
			return true
		}
	}
	return false
}

// onNodeTombstoneUpdated acts on a tombstone of a path changing.
//
// A live holder of the node is cleared once its instance is gone: no config
// file and no config manager. It is decided here, the discovery being the
// only installer of a config fetched, so a fetch in flight does not race it:
// the fetch is cancelled first.
//
// A holder cleared with a config published, as one kept, is a source again:
// its config was ignored while it was pending.
func (t *Manager) onNodeTombstoneUpdated(c *msgbus.NodeTombstoneUpdated) {
	p, err := naming.ParsePath(c.Path)
	if err != nil {
		return
	}
	if c.Value.IsPending(t.localhost) {
		t.cancelFetcher(c.Path)
		t.clearLiveHolderIfGone(p)
		return
	}
	if t.isTombstoned(p, "") {
		return
	}
	for n, h := range c.Value.Holders {
		if n == t.localhost || h.State != tombstone.HolderCleared {
			continue
		}
		if cfg := instance.ConfigData.GetByPathAndNode(p, n); cfg != nil {
			t.onRemoteConfigUpdated(p, n, *cfg)
		}
	}
}

// clearLiveHolderIfGone clears the live holder of the node in the tombstones
// of p once its instance is gone there.
func (t *Manager) clearLiveHolderIfGone(p naming.Path) {
	if _, ok := t.cfgMTime[p.String()]; ok {
		// A config manager runs: its end tells.
		return
	}
	if file.Exists(p.ConfigFile()) {
		return
	}
	for _, rec := range tombstone.Data.MergedOf(p.String()) {
		h, ok := rec.Holders[t.localhost]
		if !ok || h.State != tombstone.HolderPending || h.Mode != tombstone.HolderLive {
			continue
		}
		if err := clearLocalTombstoneFunc(p.String(), rec.Kind.GoneBy()); err != nil {
			t.objectLogger(p).Warnf("cfg: clear the holder %s of the tombstones of %s: %s", t.localhost, p, err)
		}
		return
	}
}
