package tmplrender

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/clusterdump"
)

type (
	// State is what a render reads besides the fetched values: the
	// cluster data the events keep, as the node serving the events sees
	// it, and the checksums of the resource informations, which no
	// cluster data holds.
	State struct {
		Data *clusterdump.Data

		// ResourceInfo are the checksums of the resource informations
		// of the instances, by ResourceInfoID.
		ResourceInfo map[string]string
	}

	// Read is a read of a render: the events telling what it read may
	// have changed, and its fingerprint, of what it read, or of the
	// version of what it fetched.
	Read struct {
		// ID names the read, as the template wrote it: avail
		// prod/svc/web.
		ID string

		// Kinds are the kinds of the events telling the read may have
		// changed.
		Kinds []string

		// Selection is the path, or path pattern, of the objects the
		// read follows the events of. It is empty for a read of node
		// data, whose events name no object.
		Selection string

		// AllObjects is true for a read following the events of every
		// object, as a range with no selector.
		AllObjects bool

		probe       func(*State) string
		fingerprint string
	}

	// Subscription is the events a set of reads needs: the kinds, of the
	// objects the selector selects, or of all of them when it is empty.
	Subscription struct {
		Kinds    []string
		Selector string
	}
)

// NewState returns a state on the cluster data data.
func NewState(data *clusterdump.Data) *State {
	return &State{Data: data, ResourceInfo: make(map[string]string)}
}

// ResourceInfoID is the key of the checksum of the resource information of
// the instance of path on node in State.ResourceInfo.
func ResourceInfoID(path, node string) string {
	return path + "@" + node
}

// servedBy is the node whose cluster data the state holds.
func (t *State) servedBy() string {
	return t.Data.Daemon.Nodename
}

// Changed returns the ids of the reads whose fingerprint in st is not the one
// they had at the render.
func Changed(reads []Read, st *State) []string {
	var l []string
	for _, r := range reads {
		if r.probe(st) != r.fingerprint {
			l = append(l, r.ID)
		}
	}
	return l
}

// SubscriptionOf returns the events the reads need.
func SubscriptionOf(reads []Read) Subscription {
	var (
		kinds      = make(map[string]bool)
		selections = make(map[string]bool)
		all        bool
	)
	for _, r := range reads {
		for _, k := range r.Kinds {
			kinds[k] = true
		}
		switch {
		case r.AllObjects:
			all = true
		case r.Selection != "":
			selections[r.Selection] = true
		}
	}
	s := Subscription{Kinds: sortedKeys(kinds)}
	if !all && len(selections) > 0 {
		// The selector of the stream is the union of the selections,
		// each a term, or terms joined by +, of the selector syntax.
		s.Selector = strings.Join(sortedKeys(selections), ",")
	}
	return s
}

// Equal returns true when s and o ask the same events.
func (t Subscription) Equal(o Subscription) bool {
	return t.Selector == o.Selector && slices.Equal(t.Kinds, o.Kinds)
}

// Filters returns the filters of an events stream asking the events of s.
func (t Subscription) Filters() []string {
	return slices.Clone(t.Kinds)
}

func sortedKeys(m map[string]bool) []string {
	l := make([]string, 0, len(m))
	for k := range m {
		l = append(l, k)
	}
	sort.Strings(l)
	return l
}

// fingerprint returns a hash of the json form of v.
func fingerprint(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// Not expected from the data a template reads: a value
		// never equal to another forces a render.
		return "error:" + err.Error()
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
