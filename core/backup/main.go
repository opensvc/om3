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
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	// Run is one sync update of one object, as a store holds it.
	Run struct {
		// ID is om's snapshot id: the session id of the action that
		// wrote the run, shared by every member.
		ID string `json:"id"`

		// Path is the object the run backs up.
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

		// Members are the members the run saved.
		Members []Member `json:"members"`
	}

	// Member is a part of a run: a head saved from a directory, or a
	// document saved from a stream.
	Member struct {
		Name string     `json:"name"`
		Kind MemberKind `json:"kind"`
	}

	// MemberKind says how a member was saved, and so how it is restored.
	MemberKind string

	// RunFilter selects runs. A zero field selects any.
	RunFilter struct {
		// Path selects the runs of an object.
		Path string

		// ID selects the run whose id is, or starts with, ID.
		ID string
	}

	// Store is what every store driver does.
	Store interface {
		// Ref is the reference to the store, as a store keyword or
		// the --store option names it.
		Ref() string

		// Backup saves the directory src as the member of run, without
		// the paths excludes matches, relative to src.
		Backup(ctx context.Context, run Run, member string, src string, excludes []string) error

		// Runs returns the runs filter selects, oldest first.
		Runs(ctx context.Context, filter RunFilter) ([]Run, error)

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
	}
)

const (
	// MemberDir is a member saved from a directory.
	MemberDir MemberKind = "dir"

	// MemberStream is a member saved from a stream.
	MemberStream MemberKind = "stream"

	// MemberConfig is the member holding the configuration of the object.
	MemberConfig = "config"

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

// T is the base of the store drivers: the section declaring the store, and
// the configuration holding it.
type T struct {
	name   string
	config *xconfig.T
}

// Ref returns the reference to the store: its name, the section without its
// prefix.
func (t *T) Ref() string {
	return strings.TrimPrefix(t.name, SectionPrefix)
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
	if f.Path != "" && run.Path != f.Path {
		return false
	}
	if f.ID != "" && !strings.HasPrefix(run.ID, f.ID) {
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

// FindRun returns the one run of the object at path whose id is, or starts
// with, id, or the latest run when id is empty.
func FindRun(ctx context.Context, s Store, path, id string) (Run, error) {
	runs, err := s.Runs(ctx, RunFilter{Path: path, ID: id})
	if err != nil {
		return Run{}, err
	}
	switch {
	case len(runs) == 0 && id == "":
		return Run{}, fmt.Errorf("%w: store %s holds no run of %s", ErrNotFound, s.Ref(), path)
	case len(runs) == 0:
		return Run{}, fmt.Errorf("%w: store %s holds no run %s of %s", ErrNotFound, s.Ref(), id, path)
	case id == "":
		return runs[len(runs)-1], nil
	case len(runs) > 1:
		for _, run := range runs {
			if run.ID == id {
				return run, nil
			}
		}
		return Run{}, fmt.Errorf("%w: %s matches %d runs of %s in store %s", ErrAmbiguous, id, len(runs), path, s.Ref())
	default:
		return runs[0], nil
	}
}

// FindSnapshot returns the run ref names among the runs of every object of
// the store: its id, or a prefix of it, or else an object path, meaning the
// latest run of that object. An id never holds a "/", but the path of an
// object of the root namespace has none either, so an id is tried first.
func FindSnapshot(ctx context.Context, s Store, ref string) (Run, error) {
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
	run, err := FindRun(ctx, s, ref, "")
	if errors.Is(err, ErrNotFound) {
		return Run{}, fmt.Errorf("%w: store %s holds no run %s, nor a run of an object %s", ErrNotFound, s.Ref(), ref, ref)
	}
	return run, err
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
