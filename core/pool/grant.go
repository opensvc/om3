package pool

import (
	"fmt"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	// Grant is a claim answered yes and not yet seen in the configurations
	// the cluster shares.
	Grant struct {
		// To is the size the object was let hold, which is what it will
		// claim of the pool once its configuration is written.
		To int64

		// ExpiresAt is when the grant stops being counted. A claim answered
		// yes is not always written: the write can fail after the answer, or
		// the caller can go away between the two, and a grant nothing ever
		// claims would ration the namespace for ever.
		ExpiresAt time.Time
	}

	// Grants is what a node has let namespaces take of the pools, waiting for
	// the configurations saying so to come back around.
	//
	// A claim counts the size each volume of the namespace is configured to
	// hold, read from the configurations the cluster shares. A write reaches
	// that reading a moment after it is made, so a claim answered from it
	// alone misses everything granted since: three volumes grown one after
	// the other each read the sizes of before the first grow. Counting the
	// grants too closes that window, and holding them here is what makes the
	// answer and the record one step rather than two.
	Grants struct {
		mu  sync.Mutex
		ttl time.Duration

		// seq names the grants of an object this node cannot name, so that
		// one does not take the place of another.
		seq int64

		// m is the grants of a namespace on a pool, by the object they were
		// granted for.
		m map[string]map[string]Grant
	}
)

// NewGrants returns a table forgetting a grant nothing claimed after ttl.
func NewGrants(ttl time.Duration) *Grants {
	return &Grants{
		ttl: ttl,
		m:   make(map[string]map[string]Grant),
	}
}

// Fits says whether the namespace may have an object hold to bytes of a pool,
// and records the grant when it may.
//
// held is the size each volume of the namespace is configured to hold of the
// pool, by object path, which is the reading the grants complete.
//
// What is weighed is what the namespace would hold once the write lands:
// every other object counts for the larger of what its configuration says and
// what it was let take, and the object asked about counts for the size asked
// for. So asking twice for the same size is answered the same way twice, and
// a grant stops adding anything as soon as the configuration it was granted
// for says the same thing.
func (t *Grants) Fits(namespace, poolName, path string, to, limit int64, held map[string]int64, now time.Time) (bool, string) {
	return t.fits(namespace, poolName, path, to, limit, held, now, true)
}

// Probe says whether the namespace may have an object hold to bytes of a
// pool, and records nothing.
//
// It is what a pool lookup asks of every pool it weighs: a claim is taken
// where a volume is written, and a pool the lookup only looked at is a pool
// nothing was written to. Recording there would ration the namespace on every
// pool it was compared against, for as long as a grant lasts.
func (t *Grants) Probe(namespace, poolName, path string, to, limit int64, held map[string]int64, now time.Time) (bool, string) {
	return t.fits(namespace, poolName, path, to, limit, held, now, false)
}

func (t *Grants) fits(namespace, poolName, path string, to, limit int64, held map[string]int64, now time.Time, record bool) (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := namespace + "\x00" + poolName
	grants, ok := t.m[key]
	if !ok {
		grants = make(map[string]Grant)
		t.m[key] = grants
	}

	// Forget the grants the configurations have caught up with, and the ones
	// nothing was written for.
	for p, g := range grants {
		if now.After(g.ExpiresAt) || held[p] >= g.To {
			delete(grants, p)
		}
	}

	total := to
	for p, size := range held {
		if p == path {
			continue
		}
		if g, ok := grants[p]; ok && g.To > size {
			size = g.To
		}
		total += size
	}
	for p, g := range grants {
		if p == path {
			continue
		}
		if _, ok := held[p]; ok {
			// Counted above, at the larger of the two.
			continue
		}
		total += g.To
	}

	if total > limit {
		current := held[path]
		return false, fmt.Sprintf("the %s namespace may claim %s of it and already claims %s, so it cannot claim %s more",
			namespace,
			sizeconv.BSizeCompact(float64(limit)),
			sizeconv.BSizeCompact(float64(total-to+current)),
			sizeconv.BSizeCompact(float64(to-current)))
	}
	if !record || to <= 0 {
		// A lookup asking for nothing takes nothing, and a grant of nothing
		// would only be something to forget later.
		return true, ""
	}
	if path == "" {
		// An object with no name yet is one no configuration will ever
		// report the grant of, so it is kept under a name of its own and
		// forgotten when it expires.
		t.seq++
		path = fmt.Sprintf("\x00%d", t.seq)
	}
	grants[path] = Grant{To: to, ExpiresAt: now.Add(t.ttl)}
	return true, ""
}

// Seed records a grant this node did not answer.
//
// It is how a table lost is rebuilt. The grants of a node are in memory, so a
// node that has just started, or has just become the one answering claims,
// has none of them and answers from the configurations the cluster shares
// alone, which is the reading the grants exist to complete.
//
// What was granted and written is not lost, though: it is on the node that
// wrote it, whose own reading of it has no lag. What was granted and never
// written is lost, and expiring is what would have become of it anyway.
//
// A grant already held for the object is left alone unless the seed is
// larger, so rebuilding cannot lower what a namespace is counted as holding.
func (t *Grants) Seed(namespace, poolName, path string, to int64, now time.Time) {
	if path == "" || to <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	key := namespace + "\x00" + poolName
	grants, ok := t.m[key]
	if !ok {
		grants = make(map[string]Grant)
		t.m[key] = grants
	}
	if g, ok := grants[path]; ok && g.To >= to {
		return
	}
	grants[path] = Grant{To: to, ExpiresAt: now.Add(t.ttl)}
}

// ExpiresAt is when a grant made now stops being counted.
func (t *Grants) ExpiresAt(now time.Time) time.Time {
	return now.Add(t.ttl)
}
