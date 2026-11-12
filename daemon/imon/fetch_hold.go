package imon

import (
	"time"
)

// fetchHoldTimeout is how long the orchestration holds its actions for the
// resource files fetch and ingest running.
const fetchHoldTimeout = time.Minute

// holdForFetch says whether the orchestration holds its actions for the fetch
// of the resource files of a peer, and the ingest closing it, running.
//
// The ingest is an action of the instance, holding its lock: an action the
// orchestration starts beside it, as the provision of a non-leader the leader
// coming up provisioned allows, the event which also makes its files
// fetchable, waits for the lock and fails on its timeout when the ingest
// lasts. The orchestration waits for the fetch to end, which resumes it, up
// to fetchHoldTimeout from the first of the fetches running one after the
// other: past it, the actions go on, and wait on the lock for the ingest.
func (t *Manager) holdForFetch() bool {
	if t.files == nil || !t.files.fetching || t.files.fetchingSince.IsZero() {
		return false
	}
	waited := time.Since(t.files.fetchingSince)
	if waited >= fetchHoldTimeout {
		if t.files.held {
			t.log.Infof("the resource files fetch still runs after %s: orchestrate", waited.Round(time.Second))
			t.files.held = false
		}
		return false
	}
	if !t.files.held {
		t.log.Infof("hold the orchestration for the resource files fetch and ingest running, %s at most", fetchHoldTimeout)
		t.files.held = true
	}
	if t.fetchHoldTimer != nil {
		t.fetchHoldTimer.Reset(fetchHoldTimeout - waited)
	}
	return true
}
