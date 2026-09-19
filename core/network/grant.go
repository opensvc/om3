package network

import (
	"fmt"
	"sync"
	"time"
)

type (
	// Grants is the addresses a node has let namespaces take of the
	// networks, waiting for the cluster to report them.
	//
	// A claim counts the addresses the namespace holds, read from what the
	// objects of the cluster publish. An object publishes its address after
	// it has taken it, so a claim answered from that reading alone misses
	// every address taken since: two resources reserving at once are each
	// answered as though the other had not. Counting the grants too closes
	// that window, and holding them here is what makes the answer and the
	// record one step rather than two.
	//
	// A grant is held for the reservation it was granted for, which is a
	// resource of an object, so that the cluster reporting an address for
	// that resource is what releases it.
	Grants struct {
		mu  sync.Mutex
		ttl time.Duration

		// m is the grants of a namespace on a network, by the reservation
		// they were granted for.
		m map[string]map[string]time.Time
	}
)

// NewGrants returns a table forgetting a grant nothing reserved after ttl.
func NewGrants(ttl time.Duration) *Grants {
	return &Grants{
		ttl: ttl,
		m:   make(map[string]map[string]time.Time),
	}
}

// Fits says whether a namespace may take one more address of a network for a
// reservation, and records the grant when it may.
//
// held is how many addresses of the network the namespace is known to hold,
// and seen is the reservations those addresses are held for. A reservation
// already holding an address takes no new one: an object failing over to
// another node reserves the address it already has, and a claim asked twice
// for the same resource is the same address twice.
func (t *Grants) Fits(namespace, networkName, key string, limit, held int, seen map[string]bool, now time.Time) (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	tableKey := namespace + "\x00" + networkName
	grants, ok := t.m[tableKey]
	if !ok {
		grants = make(map[string]time.Time)
		t.m[tableKey] = grants
	}

	// Forget the grants the cluster has caught up with, and the ones nothing
	// was reserved for.
	for k, expiresAt := range grants {
		if now.After(expiresAt) || seen[k] {
			delete(grants, k)
		}
	}

	count := held
	for k := range grants {
		if k != key {
			count++
		}
	}
	if !seen[key] {
		count++
	}
	if count > limit {
		return false, fmt.Sprintf("the %s namespace may hold %d address(es) of it and already holds %d",
			namespace, limit, count-1)
	}
	if !seen[key] {
		grants[key] = now.Add(t.ttl)
	}
	return true, ""
}

// ExpiresAt is when a grant made now stops being counted.
func (t *Grants) ExpiresAt(now time.Time) time.Time {
	return now.Add(t.ttl)
}
