package daemonapi

import (
	"context"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	// storeStatsEntry is what the daemon knows of the figures of a store.
	storeStatsEntry struct {
		stats     *backup.Stats
		nodeStats map[string]backup.Stats
		objects   []backup.ObjectStats

		refreshing bool
		done       chan struct{}
	}
)

const (
	// storeStatsInterval is how old the figures of a store get before a
	// request for them has them computed again, in the background.
	storeStatsInterval = time.Minute

	// storeStatsTimeout bounds a computation, a store unreachable included.
	storeStatsTimeout = 2 * time.Minute
)

var storeStatsCache = struct {
	sync.Mutex
	m map[string]*storeStatsEntry
}{m: make(map[string]*storeStatsEntry)}

// cachedStoreStats returns the figures of the store ref the daemon keeps,
// and has them computed again in the background when they are older than
// storeStatsInterval: a request never waits on a store, but the first one,
// which waits for wait at most, for a page not to open empty.
//
// The figures are the sum of the stats each object of the store records,
// one read per object. A store not reachable keeps its last figures, with
// the error, and when it was last reached.
func cachedStoreStats(ref string, wait time.Duration) storeStatsEntry {
	storeStatsCache.Lock()
	e, ok := storeStatsCache.m[ref]
	if !ok {
		e = &storeStatsEntry{}
		storeStatsCache.m[ref] = e
	}
	stale := e.stats == nil || time.Since(e.stats.UpdatedAt) > storeStatsInterval
	if stale && !e.refreshing {
		e.refreshing = true
		e.done = make(chan struct{})
		go refreshStoreStats(ref, e.done)
	}
	done := e.done
	first := e.stats == nil
	storeStatsCache.Unlock()
	if first && wait > 0 && done != nil {
		select {
		case <-done:
		case <-time.After(wait):
		}
	}
	storeStatsCache.Lock()
	defer storeStatsCache.Unlock()
	return *e
}

func refreshStoreStats(ref string, done chan struct{}) {
	defer close(done)
	ctx, cancel := context.WithTimeout(context.Background(), storeStatsTimeout)
	defer cancel()
	var (
		stats     backup.Stats
		objects   []backup.ObjectStats
		nodeStats map[string]backup.Stats
	)
	n, err := object.NewNode(object.WithVolatile(true))
	if err == nil {
		var s backup.Driver
		if s, err = n.Store(ref, naming.Path{}); err == nil {
			stats, objects = backup.ComputeStats(ctx, s)
			if ns, ok := s.(backup.NodeStatser); ok {
				nodeStats = ns.NodeStats(ctx)
			}
			_ = s.Close()
		}
	}
	if err != nil {
		stats = backup.Stats{Clusters: make([]backup.ClusterStats, 0), UpdatedAt: time.Now().UTC(), CheckError: err.Error()}
	}
	storeStatsCache.Lock()
	defer storeStatsCache.Unlock()
	e := storeStatsCache.m[ref]
	e.refreshing = false
	if stats.CheckError != "" && objects == nil && e.stats != nil {
		// Unreachable: the last figures stand, saying so.
		kept := *e.stats
		kept.CheckError = stats.CheckError
		e.stats = &kept
		return
	}
	e.stats, e.objects, e.nodeStats = &stats, objects, nodeStats
}

func apiStoreStats(s backup.Stats) api.StoreStats {
	r := api.StoreStats{
		Runs:         s.Runs,
		Incomplete:   s.Incomplete,
		Protected:    s.Protected,
		Objects:      s.Objects,
		LogicalBytes: s.LogicalBytes,
		StoredBytes:  s.StoredBytes,
		FreeBytes:    s.FreeBytes,
		UpdatedAt:    s.UpdatedAt,
		Clusters:     make([]api.StoreClusterStats, 0, len(s.Clusters)),
	}
	r.FirstCreatedAt = timePtr(s.FirstCreatedAt)
	r.LastCreatedAt = timePtr(s.LastCreatedAt)
	r.CheckedAt = timePtr(s.CheckedAt)
	if s.CheckError != "" {
		r.CheckError = &s.CheckError
	}
	for _, c := range s.Clusters {
		r.Clusters = append(r.Clusters, api.StoreClusterStats{
			ClusterId:     c.ClusterID,
			ClusterName:   c.ClusterName,
			Runs:          c.Runs,
			Incomplete:    c.Incomplete,
			Protected:     c.Protected,
			Objects:       c.Objects,
			LastCreatedAt: timePtr(c.LastCreatedAt),
			LogicalBytes:  c.LogicalBytes,
		})
	}
	return r
}

func apiStoreObjectStats(o backup.ObjectStats) api.StoreObjectStats {
	r := api.StoreObjectStats{
		ClusterId:      o.ClusterID,
		ClusterName:    o.ClusterName,
		ObjectId:       o.ObjectID,
		Path:           o.Path,
		Runs:           o.Runs,
		Incomplete:     o.Incomplete,
		Protected:      o.Protected,
		Nodes:          o.Nodes,
		FirstCreatedAt: timePtr(o.FirstCreatedAt),
		LastCreatedAt:  timePtr(o.LastCreatedAt),
		LogicalBytes:   o.LogicalBytes,
		UpdatedAt:      o.UpdatedAt,
	}
	if r.Nodes == nil {
		r.Nodes = make([]string, 0)
	}
	if o.StoredBytes > 0 {
		r.StoredBytes = &o.StoredBytes
	}
	return r
}

// timePtr returns nil for the zero time, which the api leaves out.
func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
