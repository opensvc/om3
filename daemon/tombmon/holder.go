package tombmon

import (
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/session"
	"github.com/opensvc/om3/v3/util/file"
)

// evaluateLocal looks at the away holder of the node in the records of
// path, or of every path for an empty one, once the node monitor is out of
// its start and rejoin. The caller holds mu.
//
// The holder is cleared as purged or deleted once the instance is gone
// there: its config file and config data absent, as the operator purged it.
// It is cleared as kept once its config file changes, the operator editing
// or touching it to keep the instance: its config then propagates again.
//
// A change is a keep only if the instance is quiet: its monitor not
// purging, deleting or unprovisioning, idle, and no action of it running,
// as one rewriting the config. A change made otherwise takes the baseline
// again. The baseline is the modification time of the config file the
// first time it is looked at, in memory: a daemon restart takes it again,
// which never reads a keep where there was none. No time of another node is
// compared.
//
// A live holder is not cleared here: the discovery, the only installer of
// a config fetched, clears it, so a fetch in flight does not race it.
func (s *state) evaluateLocal(path string) {
	if !s.localRejoined() {
		return
	}
	records := s.recordsOf(path)
	if path == "" {
		records = make([]tombstone.Record, 0, len(s.records))
		for _, rec := range s.records {
			records = append(records, rec)
		}
	}
	for _, rec := range records {
		id := rec.OrchestrationID
		h, ok := rec.Holders[s.localhost]
		if !ok || h.State != tombstone.HolderPending || h.Mode != tombstone.HolderAway {
			delete(s.baseline, id)
			continue
		}
		p, err := naming.ParsePath(rec.Path)
		if err != nil {
			continue
		}
		mtime := file.ModTime(p.ConfigFile())
		if mtime.IsZero() && instance.ConfigData.GetByPathAndNode(p, s.localhost) == nil {
			s.clearLocal(rec, rec.Kind.GoneBy())
			continue
		}
		base, ok := s.baseline[id]
		if !ok {
			s.baseline[id] = mtime
			continue
		}
		if mtime.Equal(base) {
			continue
		}
		if !s.quiet(p) {
			s.baseline[id] = mtime
			continue
		}
		s.log.Infof("tombstone %s of %s: the config changed on this node: kept", rec.OrchestrationID, rec.Path)
		s.clearLocal(rec, tombstone.ClearedByKept)
	}
}

// quiet says whether nothing but the operator may be changing the config of
// the local instance of p.
func (s *state) quiet(p naming.Path) bool {
	if mon := instance.MonitorData.GetByPathAndNode(p, s.localhost); mon != nil {
		switch mon.GlobalExpect {
		case instance.MonitorGlobalExpectPurged, instance.MonitorGlobalExpectDeleted, instance.MonitorGlobalExpectUnprovisioned:
			return false
		}
		if mon.State != instance.MonitorStateIdle {
			return false
		}
	}
	running := session.ListExecs(session.Filter{
		Path:   p.String(),
		Node:   s.localhost,
		States: []session.State{session.StateRunning},
	})
	for _, e := range running {
		if !readsOnly(e.Command) {
			return false
		}
	}
	return true
}

// readsOnly says whether the exec of command only reads the instance, as
// the status refresh the imon runs on a change of the config, which a keep
// by a touch itself triggers: it would else never read as quiet.
func readsOnly(command string) bool {
	return slices.Contains(strings.Fields(command), "status")
}

// clearLocal clears the holder of the node in rec as by. The caller holds
// mu.
func (s *state) clearLocal(rec tombstone.Record, by tombstone.ClearedBy) {
	rec = rec.DeepCopy()
	h := rec.Holders[s.localhost]
	h.State = tombstone.HolderCleared
	h.ClearedBy = by
	rec.Holders[s.localhost] = h
	delete(s.baseline, rec.OrchestrationID)
	s.upsert(rec)
	s.log.Infof("tombstone %s of %s: holder %s cleared, %s", rec.OrchestrationID, rec.Path, s.localhost, by)
	s.removeIfDone(rec.OrchestrationID)
}
