package schedule

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/key"
	usched "github.com/opensvc/om3/v3/util/schedule"
)

type (
	Table []Entry

	Config struct {
		Action             string `json:"action"`
		Schedule           string `json:"schedule"`
		Key                string `json:"key"`
		MaxParallel        int    `json:"max_parallel"`
		Require            string `json:"require,omitempty"`
		RequireCollector   bool   `json:"require_collector"`
		RequireProvisioned bool   `json:"require_provisioned"`
		RunDir             string `json:"-"`

		// RequireReplicationSource schedules the action on the node the
		// data is replicated from only.
		RequireReplicationSource bool `json:"require_replication_source,omitempty"`

		// StatefileKey is used in the last run filename and last run success formatters.
		// Defaults to Action if empty.
		StatefileKey string `json:"-"`
	}

	Entry struct {
		Config
		LastRunAt time.Time   `json:"last_run_at"`
		NextRunAt time.Time   `json:"next_run_at"`
		Node      string      `json:"node"`
		Path      naming.Path `json:"path"`
	}
)

func NewTable(entries ...Entry) Table {
	t := make([]Entry, 0)
	return Table(t).AddEntries(entries...)
}

func (t Table) Merge(i interface{}) Table {
	switch o := i.(type) {
	case Table:
		return t.MergeEntries(o...)
	case Entry:
		return t.MergeEntries(o)
	case []Entry:
		return t.MergeEntries(o...)
	default:
		return t
	}
}

func (t Table) Add(i interface{}) Table {
	switch o := i.(type) {
	case Table:
		return t.AddTable(o)
	case Entry:
		return t.AddEntries(o)
	case []Entry:
		return t.AddEntries(o...)
	default:
		return t
	}
}

func (t Table) AddTable(l Table) Table {
	return append(t, l...)
}

func (t Table) MergeEntry(e Entry) Table {
	for i, x := range t {
		if (x.Path == e.Path) && (x.Node == e.Node) && (x.Key == e.Key) {
			t[i] = e
			return t
		}
	}
	return append(t, e)
}

func (t Table) DelEntry(e Entry) Table {
	for i, x := range t {
		if (x.Path == e.Path) && (x.Node == e.Node) && (x.Key == e.Key) {
			return append(t[:i], t[i+1:]...)
		}
	}
	return t
}

func (t Table) MergeEntries(l ...Entry) Table {
	for _, e := range l {
		t = t.MergeEntry(e)
	}
	return t
}

func (t Table) AddEntries(l ...Entry) Table {
	return append(t, l...)
}

func (t Table) DeepCopy() *Table {
	r := make(Table, 0, len(t))
	for _, x := range t {
		r = append(r, Entry{
			Config: Config{
				Action:             x.Action,
				Key:                x.Key,
				MaxParallel:        x.MaxParallel,
				Require:            x.Require,
				RequireCollector:   x.RequireCollector,
				RequireProvisioned: x.RequireProvisioned,
				RunDir:             x.RunDir,

				RequireReplicationSource: x.RequireReplicationSource,
				Schedule:                 x.Schedule,
				StatefileKey:             x.StatefileKey,
			},
			LastRunAt: x.LastRunAt,
			NextRunAt: x.NextRunAt,
			Node:      x.Node,
			Path:      x.Path,
		})
	}
	return &r
}

func (t Entry) GetNext() (time.Time, time.Duration, error) {
	return t.GetNextAt(time.Now())
}

// GetNextAt returns the next time the entry is allowed to run after tm, given
// its last run, and the duration it stays allowed.
//
// A caller computing a delay from a time it read passes that time, so the
// delay and the next date are computed from the same clock reading.
func (t Entry) GetNextAt(tm time.Time) (time.Time, time.Duration, error) {
	sc := usched.New(t.Schedule)
	return sc.Next(usched.NextWithLast(t.LastRunAt), usched.NextWithTime(tm))
}

func (t Entry) LogPrefix() string {
	var s strings.Builder

	if t.Path.IsZero() {
		s.WriteString("node: ")
	} else {
		s.WriteString(t.Path.String())
		s.WriteString(": ")
	}

	if rid := t.RID(); rid != "DEFAULT" {
		s.WriteString(rid)
		s.WriteString(": ")
	}
	s.WriteString(t.Action)
	s.WriteString(": ")
	return s.String()
}

func (t *Entry) RID() string {
	k := key.Parse(t.Key)
	return k.Section
}

func (t *Entry) SetLastSuccess(tm time.Time) error {
	return file.Touch(t.LastSuccessFile(), tm)
}

func (t *Entry) SetLastRun(tm time.Time) error {
	return file.Touch(t.LastRunFile(), tm)
}

func (t *Entry) GetLastRun() time.Time {
	return file.ModTime(t.LastRunFile())
}

func (t *Entry) LastRunFile() string {
	return filepath.Join(t.Path.VarDir(), "scheduler", t.StatefileKey)
}

func (t *Entry) LastSuccessFile() string {
	return t.LastRunFile() + ".success"
}

func (t *Entry) LoadLast() time.Time {
	fpath := t.LastRunFile()
	return file.ModTime(fpath)
}
