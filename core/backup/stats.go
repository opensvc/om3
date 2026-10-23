package backup

import (
	"context"
	"slices"
	"time"
)

type (
	// Stats are the figures of a store, summed from the stats every object
	// of it records: every cluster reading the store gets the same ones,
	// the runs of every cluster writing it counted.
	Stats struct {
		// Runs are the completed runs, and Incomplete the ones a failed
		// or interrupted update left.
		Runs       int `json:"runs"`
		Incomplete int `json:"incomplete"`
		Protected  int `json:"protected"`

		// Objects are the objects with a run, counted by object id.
		Objects int `json:"objects"`

		FirstCreatedAt time.Time `json:"first_created_at,omitzero"`
		LastCreatedAt  time.Time `json:"last_created_at,omitzero"`

		// LogicalBytes is the size of what the runs saved. StoredBytes
		// is what they take in the store, nil when the driver can not
		// tell, and FreeBytes the space left there, nil for a store not
		// telling, as an object storage.
		LogicalBytes int64  `json:"logical_bytes"`
		StoredBytes  *int64 `json:"stored_bytes,omitempty"`
		FreeBytes    *int64 `json:"free_bytes,omitempty"`

		// CheckedAt is when the store was last reached, and CheckError
		// why the last attempt failed to reach it.
		CheckedAt  time.Time `json:"checked_at,omitzero"`
		CheckError string    `json:"check_error,omitempty"`

		// UpdatedAt is when these figures were computed.
		UpdatedAt time.Time `json:"updated_at"`

		// Clusters break the figures down by the cluster that wrote the
		// runs.
		Clusters []ClusterStats `json:"clusters"`
	}

	// ClusterStats are the figures of the runs one cluster wrote.
	ClusterStats struct {
		ClusterID     string    `json:"cluster_id"`
		ClusterName   string    `json:"cluster_name"`
		Runs          int       `json:"runs"`
		Incomplete    int       `json:"incomplete"`
		Protected     int       `json:"protected"`
		Objects       int       `json:"objects"`
		LastCreatedAt time.Time `json:"last_created_at,omitzero"`
		LogicalBytes  int64     `json:"logical_bytes"`
	}

	// ObjectStats are the figures of the runs of one object, which the
	// node completing or removing a run of it records beside its runs.
	ObjectStats struct {
		ClusterID      string    `json:"cluster_id"`
		ClusterName    string    `json:"cluster_name"`
		ObjectID       string    `json:"object_id"`
		Path           string    `json:"path"`
		Runs           int       `json:"runs"`
		Incomplete     int       `json:"incomplete"`
		Protected      int       `json:"protected"`
		Nodes          []string  `json:"nodes"`
		FirstCreatedAt time.Time `json:"first_created_at,omitzero"`
		LastCreatedAt  time.Time `json:"last_created_at,omitzero"`
		LogicalBytes   int64     `json:"logical_bytes"`
		StoredBytes    int64     `json:"stored_bytes,omitempty"`
		UpdatedAt      time.Time `json:"updated_at"`
	}

	// Usage is the space a store takes and has left, nil when it can not
	// tell.
	Usage struct {
		StoredBytes *int64
		FreeBytes   *int64
	}

	// Usager is a store telling the space it takes or has left.
	Usager interface {
		Usage(ctx context.Context) (Usage, error)
	}

	// NodeStatser is a store keeping its runs in a directory of each node,
	// whose stats are of each node, by node name.
	NodeStatser interface {
		NodeStats(ctx context.Context) map[string]Stats
	}

	// Statser is a store reading the stats every object of it records,
	// of every cluster, one record per object rather than a read of every
	// run.
	Statser interface {
		ObjectStats(ctx context.Context) ([]ObjectStats, error)
	}
)

// NewObjectStats counts the runs of one object, the incomplete ones too.
func NewObjectStats(clusterID, clusterName, objectID, path string, runs []Run) ObjectStats {
	o := ObjectStats{
		ClusterID:   clusterID,
		ClusterName: clusterName,
		ObjectID:    objectID,
		Path:        path,
		Nodes:       make([]string, 0),
		UpdatedAt:   time.Now().UTC(),
	}
	for _, run := range runs {
		if run.CompletedAt.IsZero() {
			o.Incomplete++
			continue
		}
		o.Runs++
		if run.Protected {
			o.Protected++
		}
		if run.Node != "" && !slices.Contains(o.Nodes, run.Node) {
			o.Nodes = append(o.Nodes, run.Node)
		}
		if o.FirstCreatedAt.IsZero() || run.CreatedAt.Before(o.FirstCreatedAt) {
			o.FirstCreatedAt = run.CreatedAt.UTC()
		}
		if run.CreatedAt.After(o.LastCreatedAt) {
			o.LastCreatedAt = run.CreatedAt.UTC()
		}
		o.LogicalBytes += run.LogicalBytes
		o.StoredBytes += run.StoredBytes
	}
	slices.Sort(o.Nodes)
	return o
}

// SumStats sums the stats of the objects of a store. An object counts once
// it holds a run, a completed one or not. The stored bytes are summed when
// every object with a completed run tells them.
func SumStats(objects []ObjectStats) Stats {
	s := Stats{Clusters: make([]ClusterStats, 0), UpdatedAt: time.Now().UTC()}
	byCluster := make(map[string]*ClusterStats)
	var stored int64
	storedKnown := true
	for _, o := range objects {
		if o.Runs+o.Incomplete == 0 {
			continue
		}
		s.Runs += o.Runs
		s.Incomplete += o.Incomplete
		s.Protected += o.Protected
		s.Objects++
		s.LogicalBytes += o.LogicalBytes
		if o.Runs > 0 && o.StoredBytes == 0 {
			// Saved by a driver not telling, or before the sizes
			// were recorded: a run stores a few bytes at least.
			storedKnown = false
		}
		stored += o.StoredBytes
		if !o.FirstCreatedAt.IsZero() && (s.FirstCreatedAt.IsZero() || o.FirstCreatedAt.Before(s.FirstCreatedAt)) {
			s.FirstCreatedAt = o.FirstCreatedAt
		}
		if o.LastCreatedAt.After(s.LastCreatedAt) {
			s.LastCreatedAt = o.LastCreatedAt
		}
		c, ok := byCluster[o.ClusterID]
		if !ok {
			c = &ClusterStats{ClusterID: o.ClusterID}
			byCluster[o.ClusterID] = c
		}
		if c.ClusterName == "" {
			c.ClusterName = o.ClusterName
		}
		c.Runs += o.Runs
		c.Incomplete += o.Incomplete
		c.Protected += o.Protected
		c.Objects++
		c.LogicalBytes += o.LogicalBytes
		if o.LastCreatedAt.After(c.LastCreatedAt) {
			c.LastCreatedAt = o.LastCreatedAt
		}
	}
	if storedKnown && s.Objects > 0 {
		s.StoredBytes = &stored
	}
	for _, c := range byCluster {
		s.Clusters = append(s.Clusters, *c)
	}
	slices.SortFunc(s.Clusters, func(a, b ClusterStats) int { return b.LastCreatedAt.Compare(a.LastCreatedAt) })
	return s
}

// StoreStats computes the stats of the store s: the stats its objects
// record, and the space it takes and has left when it tells. The store
// unreachable, the stats say why, and when it was last reached is up to the
// caller to keep.
func StoreStats(ctx context.Context, s Store) Stats {
	stats, _ := ComputeStats(ctx, s)
	return stats
}

// ComputeStats is StoreStats, returning too the stats of the objects it
// summed.
func ComputeStats(ctx context.Context, s Store) (Stats, []ObjectStats) {
	statser, ok := s.(Statser)
	if !ok {
		return Stats{Clusters: make([]ClusterStats, 0), UpdatedAt: time.Now().UTC(), CheckError: "the store keeps no stats"}, nil
	}
	objects, err := statser.ObjectStats(ctx)
	if err != nil {
		return Stats{Clusters: make([]ClusterStats, 0), UpdatedAt: time.Now().UTC(), CheckError: err.Error()}, nil
	}
	stats := SumStats(objects)
	stats.CheckedAt = stats.UpdatedAt
	if usager, ok := s.(Usager); ok {
		if u, err := usager.Usage(ctx); err != nil {
			stats.CheckError = err.Error()
		} else {
			if u.StoredBytes != nil {
				stats.StoredBytes = u.StoredBytes
			}
			stats.FreeBytes = u.FreeBytes
		}
	}
	return stats, objects
}

// MergeObjectStats merges the stats of one object recorded in several
// directories, as the one of each node of a store keeping its runs per node:
// the runs of an object moved from a node to another are in both.
func MergeObjectStats(lists ...[]ObjectStats) []ObjectStats {
	merged := make(map[string]*ObjectStats)
	order := make([]string, 0)
	for _, l := range lists {
		for _, o := range l {
			k := o.ClusterID + "/" + o.ObjectID
			m, ok := merged[k]
			if !ok {
				c := o
				c.Nodes = slices.Clone(o.Nodes)
				merged[k] = &c
				order = append(order, k)
				continue
			}
			m.Runs += o.Runs
			m.Incomplete += o.Incomplete
			m.Protected += o.Protected
			m.LogicalBytes += o.LogicalBytes
			m.StoredBytes += o.StoredBytes
			for _, n := range o.Nodes {
				if !slices.Contains(m.Nodes, n) {
					m.Nodes = append(m.Nodes, n)
				}
			}
			slices.Sort(m.Nodes)
			if !o.FirstCreatedAt.IsZero() && (m.FirstCreatedAt.IsZero() || o.FirstCreatedAt.Before(m.FirstCreatedAt)) {
				m.FirstCreatedAt = o.FirstCreatedAt
			}
			if o.LastCreatedAt.After(m.LastCreatedAt) {
				m.LastCreatedAt, m.Path = o.LastCreatedAt, o.Path
			}
			if o.UpdatedAt.After(m.UpdatedAt) {
				m.UpdatedAt = o.UpdatedAt
			}
		}
	}
	l := make([]ObjectStats, 0, len(order))
	for _, k := range order {
		l = append(l, *merged[k])
	}
	return l
}
