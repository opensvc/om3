// Package backup is the framework the sync.store resources back objects up
// with, into stores declared as store#<name> sections.
//
// A store is a tool keeping backups, plakar or a directory of tar archives,
// behind the Store interface. The framework decides what is backed up, when,
// and what is kept; a store driver only writes, lists, restores and deletes
// what it is told to.
//
// One sync update of an object is a run, identified by om's snapshot id, the
// action session id, whatever the tool: the same id is listed, restored and
// deployed from on every store type. A run holds named members: one per head
// the run saved, named by its resource id, and the configuration of the
// object, named config.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	// Run is one sync update of one object, as a store holds it.
	Run struct {
		// ID is om's snapshot id: the session id of the action that
		// wrote the run, shared by every member.
		ID string `json:"id"`

		// ClusterID and ClusterName are the cluster of the node that
		// wrote the run: a store several clusters write holds objects of
		// the same path, and of the same id, as one deployed on a drp
		// cluster with its source's id.
		ClusterID   string `json:"cluster_id"`
		ClusterName string `json:"cluster_name"`

		// Path is the object the run backs up, as it was named then.
		Path string `json:"path"`

		// ObjectID is the id of the object, telling apart two objects
		// that held the same path, as a clone deployed over a purged
		// one.
		ObjectID string `json:"object_id"`

		// Node is the node that wrote the run.
		Node string `json:"node"`

		// CreatedAt is when the run started, which the retention
		// periods are evaluated on.
		CreatedAt time.Time `json:"created_at"`

		// CompletedAt is when the run saved its last member, every
		// member saved. A run failed or interrupted has none, and no
		// listing or restore selects it.
		CompletedAt time.Time `json:"completed_at,omitzero"`

		// Members are the members the run saved.
		Members []Member `json:"members"`
	}

	// Member is a part of a run: a head saved from a directory, or a
	// document saved from a stream.
	Member struct {
		Name string     `json:"name"`
		Kind MemberKind `json:"kind"`

		// Head is the directory a member saved from a directory was
		// saved from, which a restore in place writes it back into.
		Head string `json:"head,omitempty"`
	}

	// MemberKind says how a member was saved, and so how it is restored.
	MemberKind string

	// RunFilter selects runs. A zero field selects any.
	RunFilter struct {
		// Path selects the runs of the objects the store last recorded at
		// this path, of every cluster: an object keeps its runs when it
		// is renamed, and the runs it took before still say the path it
		// had then. The store applies it, Match does not.
		Path string

		// ObjectID selects the runs of an object, whatever its path.
		ObjectID string

		// Cluster selects the runs the nodes of a cluster wrote, by the
		// cluster id or name. A store resolves a name to the cluster it
		// names now, so the runs a renamed cluster wrote are selected.
		Cluster string

		// ID selects the run whose id is, or starts with, ID.
		ID string

		// Incomplete selects the runs not completed too, as a failed
		// or interrupted run left them.
		Incomplete bool
	}

	// Store is what every store driver does.
	Store interface {
		// Ref is the reference to the store, as a store keyword or
		// the --store option names it.
		Ref() string

		// Backup saves the directory src as the member of run, without
		// the paths excludes matches, relative to src, and records head
		// as the head of the member: src itself, or the head a snapshot
		// at src is of.
		Backup(ctx context.Context, run Run, member, src, head string, excludes []string) error

		// Runs returns the runs filter selects, oldest first.
		Runs(ctx context.Context, filter RunFilter) ([]Run, error)

		// Complete marks the run completed, every member saved.
		Complete(ctx context.Context, run Run) error

		// Restore writes the member of run into the directory dst.
		Restore(ctx context.Context, run Run, member string, dst string) error

		// Check verifies the runs filter selects, the whole store
		// with a zero filter.
		Check(ctx context.Context, filter RunFilter) error
	}

	// Streamer is a store saving and reading members as streams, never
	// written to a disk on the way: the configuration of an object and
	// the keys it references.
	Streamer interface {
		BackupStream(ctx context.Context, run Run, member string, r io.Reader) error
		Cat(ctx context.Context, run Run, member string) (io.ReadCloser, error)
	}

	// Pruner is a store deleting runs, the ones the retention om applies
	// drops. A store without it keeps its runs as the backup server
	// decides, and refuses a keep.
	Pruner interface {
		Delete(ctx context.Context, runs []Run) error
	}

	// Driver is a store driver, configured from its store#<name> section.
	Driver interface {
		Store
		SetName(string)
		SetConfig(*xconfig.T)
		SetOwner(naming.Path)
		SetReader(string)
		Close() error
	}

	// Localer is a store keeping its runs on the node, as a directory.
	// A sec does not declare one: the daemon would read and write it as
	// root where the namespace says.
	Localer interface {
		IsLocal() bool
	}
)

const (
	// MemberDir is a member saved from a directory.
	MemberDir MemberKind = "dir"

	// MemberStream is a member saved from a stream.
	MemberStream MemberKind = "stream"

	// MemberConfig is the member holding the configuration of the object.
	MemberConfig = "config"

	// MemberRefs is the member holding the keys of the sec and cfg the
	// object references, as an object.RefsDocument.
	MemberRefs = "refs.json"

	// SectionPrefix is the prefix of the sections declaring stores.
	SectionPrefix = "store#"
)

var (
	// ErrNotFound is returned when no run matches a filter asking one.
	ErrNotFound = errors.New("no such run")

	// ErrAmbiguous is returned when several runs match a filter asking
	// one, as a too short id prefix.
	ErrAmbiguous = errors.New("ambiguous run id")
)

// GetDriver returns a new driver of the store type, or nil when no driver
// serves it.
func GetDriver(storeType string) Driver {
	i, ok := driver.GetStrict(driver.NewID(driver.GroupStore, storeType))
	if !ok {
		return nil
	}
	if a, ok := i.Allocator.(func() Driver); ok {
		return a()
	}
	return nil
}

// SectionName returns the section declaring the store named name, with or
// without its prefix.
func SectionName(name string) string {
	if strings.HasPrefix(name, SectionPrefix) {
		return name
	}
	return SectionPrefix + name
}

// T is the base of the store drivers: the section declaring the store, the
// configuration holding it, the sec it is of, zero for a store of the cluster
// configuration, and the namespace of the object reading it, empty for a
// node command.
type T struct {
	name   string
	config *xconfig.T
	owner  naming.Path
	reader string
}

// Ref returns the reference to the store, as a store keyword names it: its
// name, the section without its prefix, for a store of the cluster
// configuration, and from <sec> store#<name> for a store of a sec.
func (t *T) Ref() string {
	if !t.owner.IsZero() {
		return fmt.Sprintf("from %s %s", t.owner, t.name)
	}
	return strings.TrimPrefix(t.name, SectionPrefix)
}

// SetOwner sets the sec declaring the store.
func (t *T) SetOwner(p naming.Path) {
	t.owner = p
}

// Owner returns the sec declaring the store, zero for a store of the
// cluster configuration.
func (t *T) Owner() naming.Path {
	return t.owner
}

// SetReader sets the namespace of the object reading the store.
func (t *T) SetReader(ns string) {
	t.reader = ns
}

// Reader returns the namespace of the object reading the store, empty for a
// node command.
func (t *T) Reader() string {
	return t.reader
}

// Close releases what the driver holds, as a connection to the store.
func (t *T) Close() error {
	return nil
}

// SetName sets the section declaring the store.
func (t *T) SetName(s string) {
	t.name = SectionName(s)
}

// SetConfig sets the configuration holding the section.
func (t *T) SetConfig(c *xconfig.T) {
	t.config = c
}

// Config returns the configuration holding the section.
func (t *T) Config() *xconfig.T {
	return t.config
}

// Key returns the key of the option in the section of the store.
func (t *T) Key(option string) key.T {
	return key.New(t.name, option)
}

// Get returns the value of the option in the section of the store,
// evaluated, or as written when no keyword evaluates it, as in a
// configuration with no referrer.
func (t *T) Get(option string) string {
	if t.config == nil {
		return ""
	}
	k := t.Key(option)
	if v, err := t.config.GetStringStrict(k); err == nil {
		return v
	}
	return t.config.Get(k)
}

// Keep returns the retention the store gives the objects whose resource sets
// none.
func Keep(s Store) string {
	if g, ok := s.(interface{ Get(string) string }); ok {
		return g.Get("keep")
	}
	return ""
}

// Match tells whether the run is one filter selects.
func (f RunFilter) Match(run Run) bool {
	if f.ObjectID != "" && run.ObjectID != f.ObjectID {
		return false
	}
	if f.Cluster != "" && run.ClusterID != f.Cluster && run.ClusterName != f.Cluster {
		return false
	}
	if f.ID != "" && !strings.HasPrefix(run.ID, f.ID) {
		return false
	}
	if !f.Incomplete && run.CompletedAt.IsZero() {
		return false
	}
	return true
}

// Has tells whether the run saved the member.
func (t Run) Has(member string) bool {
	return slices.ContainsFunc(t.Members, func(m Member) bool { return m.Name == member })
}

// Member returns the member of the run named name.
func (t Run) Member(name string) (Member, bool) {
	i := slices.IndexFunc(t.Members, func(m Member) bool { return m.Name == name })
	if i < 0 {
		return Member{}, false
	}
	return t.Members[i], true
}

// MemberNames returns the names of the members of the run, in the order they
// were saved.
func (t Run) MemberNames() []string {
	l := make([]string, len(t.Members))
	for i, m := range t.Members {
		l[i] = m.Name
	}
	return l
}

// ShortID returns the id of the run shortened as listings show it, enough to
// select it with --snapshot in practice.
func (t Run) ShortID() string {
	if len(t.ID) > 8 {
		return t.ID[:8]
	}
	return t.ID
}

// FindRun returns the one run filter selects whose id is, or starts with,
// filter.ID, or the latest run filter selects when it names no id.
func FindRun(ctx context.Context, s Store, filter RunFilter) (Run, error) {
	runs, err := s.Runs(ctx, filter)
	if err != nil {
		return Run{}, err
	}
	switch {
	case len(runs) == 0 && filter.ID == "":
		return Run{}, fmt.Errorf("%w: store %s holds no run %s", ErrNotFound, s.Ref(), filter)
	case len(runs) == 0:
		return Run{}, fmt.Errorf("%w: store %s holds no run %s %s", ErrNotFound, s.Ref(), filter.ID, filter)
	case filter.ID == "":
		return runs[len(runs)-1], nil
	case len(runs) > 1:
		for _, run := range runs {
			if run.ID == filter.ID {
				return run, nil
			}
		}
		return Run{}, fmt.Errorf("%w: %s matches %d runs in store %s", ErrAmbiguous, filter.ID, len(runs), s.Ref())
	default:
		return runs[0], nil
	}
}

// String says what the filter selects, for a message.
func (f RunFilter) String() string {
	l := make([]string, 0, 3)
	if f.Path != "" {
		l = append(l, "of "+f.Path)
	}
	if f.ObjectID != "" {
		l = append(l, "of the object "+f.ObjectID)
	}
	if f.Cluster != "" {
		l = append(l, "of the cluster "+f.Cluster)
	}
	return strings.Join(l, " ")
}

// FindSnapshot returns the run ref names among the runs of every object of
// the store: its id, or a prefix of it, or else an object path, meaning the
// latest run of the object the store last recorded at that path. An id never
// holds a "/", but the path of an object of the root namespace has none
// either, so an id is tried first.
//
// A path the runs of several clusters hold means the latest run of the local
// cluster, cluster, or of the one other cluster holding runs of it: the
// latest of several other clusters is not one to pick for the user.
func FindSnapshot(ctx context.Context, s Store, ref, cluster string) (Run, error) {
	runs, err := s.Runs(ctx, RunFilter{ID: ref})
	if err != nil {
		return Run{}, err
	}
	for _, run := range runs {
		if run.ID == ref {
			return run, nil
		}
	}
	switch len(runs) {
	case 1:
		return runs[0], nil
	case 0:
	default:
		return Run{}, fmt.Errorf("%w: %s matches %d runs in store %s", ErrAmbiguous, ref, len(runs), s.Ref())
	}
	runs, err = s.Runs(ctx, RunFilter{Path: ref})
	if err != nil {
		return Run{}, err
	}
	if len(runs) == 0 {
		return Run{}, fmt.Errorf("%w: store %s holds no run %s, nor a run of an object %s", ErrNotFound, s.Ref(), ref, ref)
	}
	byCluster := make(map[string][]Run)
	names := make([]string, 0)
	for _, run := range runs {
		if _, ok := byCluster[run.ClusterID]; !ok {
			names = append(names, run.ClusterName)
		}
		byCluster[run.ClusterID] = append(byCluster[run.ClusterID], run)
	}
	if l, ok := byCluster[cluster]; ok {
		return l[len(l)-1], nil
	}
	if len(byCluster) > 1 {
		return Run{}, fmt.Errorf("%w: store %s holds runs of %s from the clusters %s: name the snapshot by its id", ErrAmbiguous, s.Ref(), ref, strings.Join(names, " "))
	}
	return runs[len(runs)-1], nil
}

// ReadMember returns the content of a member saved as a file: streamed from
// a store taking streams, or restored into a private temporary directory
// otherwise, where a member saved from a directory holds a file named after
// it, as the configuration of an object does.
func ReadMember(ctx context.Context, s Store, run Run, member string) ([]byte, error) {
	m, ok := run.Member(member)
	if !ok {
		return nil, fmt.Errorf("run %s of %s has no member %s", run.ShortID(), run.Path, member)
	}
	if streamer, ok := s.(Streamer); ok && m.Kind == MemberStream {
		rc, err := streamer.Cat(ctx, run, member)
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	dir, err := os.MkdirTemp("", "om-member-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if err := s.Restore(ctx, run, member, dir); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, member))
}
