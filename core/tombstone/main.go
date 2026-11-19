// Package tombstone holds the record of a delete or a purge: the version of
// the config of the object the node deleted, so a config of that version or
// older, as one a node away during the delete still holds, is known as the
// deleted one, and does not bring the object back.
//
// A record never changes. Each node writes its own, as it deletes its
// instance, keeps it as a file under its var directory and publishes it with
// its data. A node decides from its own config whether a record concerns it:
// a config newer than the version deleted, as one the operator kept by an
// edit or a touch, or one of the object created again or restored, is not
// the deleted one.
package tombstone

import (
	"time"
)

type (
	// Kind is the orchestration a record follows: a purge or a delete.
	Kind string

	// Record is the delete or the purge of the object of Path, by the
	// orchestration of OrchestrationID, on the node publishing it.
	Record struct {
		OrchestrationID string `json:"orchestration_id"`
		Path            string `json:"path"`
		Kind            Kind   `json:"kind"`

		// ObjectID is the id of the object deleted, its DEFAULT.id,
		// empty if unknown: a create of another object under the path
		// is told apart from a restore of this one.
		ObjectID string `json:"object_id,omitempty"`

		// Version is the modification time of the config file the
		// delete removed: the same on every node holding that version,
		// a fetched config keeping the time of its source.
		Version time.Time `json:"version"`

		// DeletedAt is the global expect stamp of the orchestration:
		// carried for a node completing the delete to adopt it, and
		// shown. Never compared with a time of the node.
		DeletedAt time.Time `json:"deleted_at"`

		// Nodes are the nodes that may have held the version deleted as
		// the record was written. The node removes the record early once
		// each was seen deleting its instance in the orchestration.
		Nodes []string `json:"nodes,omitempty"`
	}
)

const (
	KindPurged  Kind = "purged"
	KindDeleted Kind = "deleted"
)

// DeepCopy returns a copy of the record sharing nothing with it.
func (t Record) DeepCopy() Record {
	c := t
	if t.Nodes != nil {
		c.Nodes = append([]string{}, t.Nodes...)
	}
	return c
}

// Kills says whether a config of path modified at mtime is the deleted one,
// or an older version of it: a config newer is another, kept, created again
// or restored. A config of no known time is not judged.
func (t Record) Kills(path string, mtime time.Time) bool {
	return t.Path == path && !mtime.IsZero() && !t.Version.IsZero() && !mtime.After(t.Version)
}
