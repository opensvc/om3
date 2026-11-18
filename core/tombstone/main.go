// Package tombstone holds the record of a delete or a purge some nodes of
// the object did not run: the nodes holding the instance still, each
// pending until its instance is gone or kept. The peers fetch no
// configuration of the object from a pending holder, so the object does not
// come back from it.
//
// A holder is live when its node was up as the record was written, its
// instance not seen running the orchestration yet, as one whose config
// manager had not started: its daemon completes the delete or the purge. It
// is away when its node was not up: down, in maintenance, unknown. Its
// instance is then kept stopped, the issue shown, until the operator purges
// it there or keeps it by an edit of its configuration: never deleted on
// its own, as it may hold the only copy of the data.
//
// The records are kept as files under the var directory of the node, on
// every node that wrote or adopted one, and published with the data of the
// node: the copies are merged per holder, a cleared holder winning.
package tombstone

import (
	"slices"
	"time"
)

type (
	// Kind is the orchestration a record follows: a purge or a delete.
	Kind string

	// HolderState says whether the instance of a holder is gone or kept,
	// cleared, or held still, pending.
	HolderState string

	// HolderMode says whether the node of a holder was up as the record
	// was written, live, or not, away.
	HolderMode string

	// ClearedBy says how a holder was cleared.
	ClearedBy string

	// Holder is a node holding the instance of the object a delete or a
	// purge did not reach.
	Holder struct {
		State     HolderState `json:"state"`
		Mode      HolderMode  `json:"mode"`
		ClearedBy ClearedBy   `json:"cleared_by,omitempty"`
	}

	// Record is the tombstone of the object of Path, purged or deleted by
	// the orchestration of OrchestrationID, the key of the record: a
	// purge of an object created again with the same name is another
	// record.
	Record struct {
		OrchestrationID string `json:"orchestration_id"`
		Path            string `json:"path"`
		Kind            Kind   `json:"kind"`

		// ObjectID is the id of the object purged or deleted, empty if
		// unknown: a holder whose instance is of another object, created
		// again under the same path, holds nothing of this one.
		ObjectID string `json:"object_id,omitempty"`

		// PurgedAt is the global expect stamp of the orchestration, as
		// the node accepting it set it: carried for the live holders to
		// adopt it, the stamp the peers carry, and shown. Never compared
		// with a time of the node.
		PurgedAt time.Time `json:"purged_at"`

		Holders map[string]Holder `json:"holders"`
	}
)

const (
	KindPurged  Kind = "purged"
	KindDeleted Kind = "deleted"

	HolderPending HolderState = "pending"
	HolderCleared HolderState = "cleared"

	HolderLive HolderMode = "live"
	HolderAway HolderMode = "away"

	// ClearedByPurged and ClearedByDeleted say the instance is gone, by
	// the orchestration or by the operator.
	ClearedByPurged  ClearedBy = "purged"
	ClearedByDeleted ClearedBy = "deleted"

	// ClearedByKept says the operator kept the instance, by an edit of
	// its configuration: it propagates again.
	ClearedByKept ClearedBy = "kept"

	// ClearedByEvicted says the node left the cluster.
	ClearedByEvicted ClearedBy = "evicted"

	// ClearedByDropped says the operator dropped the record, as a create
	// --force of the same name does.
	ClearedByDropped ClearedBy = "dropped"
)

// GoneBy returns how a holder whose instance is gone is cleared, after the
// kind of the record.
func (t Kind) GoneBy() ClearedBy {
	if t == KindDeleted {
		return ClearedByDeleted
	}
	return ClearedByPurged
}

// clearedRank orders the ways a holder is cleared, for two copies cleared
// differently to merge the same on every node: the higher wins.
var clearedRank = map[ClearedBy]int{
	ClearedByKept:    5,
	ClearedByPurged:  4,
	ClearedByDeleted: 3,
	ClearedByEvicted: 2,
	ClearedByDropped: 1,
}

// DeepCopy returns a copy of the record sharing nothing with it.
func (t Record) DeepCopy() Record {
	c := t
	c.Holders = make(map[string]Holder, len(t.Holders))
	for n, h := range t.Holders {
		c.Holders[n] = h
	}
	return c
}

// HasPending says whether a holder of the record is pending.
func (t Record) HasPending() bool {
	for _, h := range t.Holders {
		if h.State == HolderPending {
			return true
		}
	}
	return false
}

// PendingHolders returns the sorted nodes of the pending holders of mode,
// or of any mode for an empty mode.
func (t Record) PendingHolders(mode HolderMode) []string {
	l := make([]string, 0)
	for n, h := range t.Holders {
		if h.State != HolderPending {
			continue
		}
		if mode != "" && h.Mode != mode {
			continue
		}
		l = append(l, n)
	}
	slices.Sort(l)
	return l
}

// OfAnotherObject says whether an instance of the object of id is another
// object than the one the record purged or deleted, created again under the
// same path. Unknown, either id empty, it is not.
func (t Record) OfAnotherObject(id string) bool {
	return t.ObjectID != "" && id != "" && t.ObjectID != id
}

// IsPending says whether node is a pending holder of the record.
func (t Record) IsPending(node string) bool {
	h, ok := t.Holders[node]
	return ok && h.State == HolderPending
}

// Merge returns the merge of two copies of a record, and whether it differs
// from a.
//
// The holders are the union of both. Of a holder in both, a cleared one
// wins over a pending one, of two cleared the higher ClearedBy wins, and
// away wins over live. The merge is commutative,
// associative and idempotent: every node merging the copies it gets, in any
// order, ends with the same record.
func Merge(a, b Record) (Record, bool) {
	m := a.DeepCopy()
	if b.Path > m.Path {
		m.Path = b.Path
	}
	if b.Kind > m.Kind {
		m.Kind = b.Kind
	}
	if b.ObjectID > m.ObjectID {
		m.ObjectID = b.ObjectID
	}
	if b.OrchestrationID > m.OrchestrationID {
		m.OrchestrationID = b.OrchestrationID
	}
	if b.PurgedAt.After(m.PurgedAt) {
		m.PurgedAt = b.PurgedAt
	}
	for n, hb := range b.Holders {
		ha, ok := m.Holders[n]
		if !ok {
			m.Holders[n] = hb
			continue
		}
		m.Holders[n] = mergeHolder(ha, hb)
	}
	return m, !m.Equal(a)
}

func mergeHolder(a, b Holder) Holder {
	m := a
	switch {
	case a.State == HolderCleared && b.State == HolderCleared:
		if clearedRank[b.ClearedBy] > clearedRank[a.ClearedBy] {
			m.ClearedBy = b.ClearedBy
		}
	case b.State == HolderCleared:
		m.State = HolderCleared
		m.ClearedBy = b.ClearedBy
	}
	if b.Mode == HolderAway {
		m.Mode = HolderAway
	}
	return m
}

// Equal says whether two records are the same.
func (t Record) Equal(o Record) bool {
	if t.OrchestrationID != o.OrchestrationID || t.Path != o.Path || t.Kind != o.Kind || t.ObjectID != o.ObjectID || !t.PurgedAt.Equal(o.PurgedAt) {
		return false
	}
	if len(t.Holders) != len(o.Holders) {
		return false
	}
	for n, h := range t.Holders {
		if oh, ok := o.Holders[n]; !ok || oh != h {
			return false
		}
	}
	return true
}
