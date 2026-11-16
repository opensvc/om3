// Package expectstamp stamps the global expects of the instance and node
// monitors.
//
// A node adopts the global expect of a peer stamped after its own, so the
// stamps order the requests: the stamp of a request is to be after every
// stamp the node accepting it has seen, which a stamp of its clock alone is
// not when its clock is behind the one of the node that accepted the
// request before: the peers then keep the request before, and the node
// adopts it back, its own request accepted and never run.
package expectstamp

import (
	"time"

	"github.com/google/uuid"
)

// Next returns the stamp of a global expect set now: the time now, or just
// after the latest of the stamps seen when the clock is behind it.
func Next(now time.Time, seen ...time.Time) time.Time {
	stamp := now.Round(0)
	for _, t := range seen {
		if !t.Before(stamp) {
			stamp = t.Add(time.Nanosecond)
		}
	}
	return stamp
}

// Newer says whether the global expect stamped at aAt, of the orchestration
// aID, is to be preferred to the one stamped at bAt, of bID: the later one,
// or of two orchestrations stamped the same, as two nodes accepting a
// request at once after the same stamp, the one of the greater id, for every
// node to keep the same one. An orchestration ended keeps its stamp and no
// id: it is no newer than the same one not ended yet, nor older.
func Newer(aAt time.Time, aID uuid.UUID, bAt time.Time, bID uuid.UUID) bool {
	if aAt.Equal(bAt) {
		if aID == uuid.Nil || bID == uuid.Nil {
			return false
		}
		return aID.String() > bID.String()
	}
	return aAt.After(bAt)
}

// NewerCandidate says whether the global expect of a peer stamped at aAt, of
// aID, is to be preferred to the one of another peer stamped at bAt, of bID,
// as the one to adopt: the later one, or of two stamped the same, the one
// running over the one ended, as one peer ended a request another runs
// still, which a node not having it yet is to run too, and of two running,
// the one of the greater id.
func NewerCandidate(aAt time.Time, aID uuid.UUID, bAt time.Time, bID uuid.UUID) bool {
	if aAt.Equal(bAt) {
		switch {
		case aID == uuid.Nil:
			return false
		case bID == uuid.Nil:
			return true
		}
		return aID.String() > bID.String()
	}
	return aAt.After(bAt)
}
